package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"path/filepath"

	"github.com/polarsignals/kubezonnet/payload"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type PodInfo struct {
	Node string
	IPs  []uint32
}

type NodeInfo struct {
	Zone string
}

type Server struct {
	clientset  *kubernetes.Clientset
	podIpIndex map[uint32]podKey // maps Pod IPv4s to Pod name
	podIndex   map[podKey]PodInfo
	nodeIndex  map[string]string // maps node name to Node zone
	statistics map[podKey]uint64
	mutex      sync.RWMutex
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Error creating kubernetes config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

	server := &Server{
		clientset:  clientset,
		podIpIndex: map[uint32]podKey{},
		podIndex:   map[podKey]PodInfo{},
		nodeIndex:  map[string]string{},
		statistics: map[podKey]uint64{},
	}

	// Start watching Pods and Nodes
	go server.watchPods()
	go server.watchNodes()

	reg := prometheus.NewRegistry()

	reg.MustRegister(server)

	http.Handle("/metrics", instrumentHandler(reg, "metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))
	http.Handle("/write-network-statistics", instrumentHandler(reg, "write_statistics", http.HandlerFunc(server.handlePayload)))
	log.Println("Starting server on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func instrumentHandler(reg prometheus.Registerer, handlerName string, handler http.Handler) http.Handler {
	reg = prometheus.WrapRegistererWith(prometheus.Labels{"handler": handlerName}, reg)

	requestsTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Tracks the number of HTTP requests.",
		}, []string{"method", "code"},
	)

	return promhttp.InstrumentHandlerCounter(requestsTotal, handler)
}

func (s *Server) watchPods() {
	watchList := cache.NewListWatchFromClient(
		s.clientset.CoreV1().RESTClient(),
		"pods",
		metav1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchList,
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onPodAdd,
			UpdateFunc: s.onPodUpdate,
			DeleteFunc: s.onPodDelete,
		},
	)
	controller.Run(make(chan struct{}))
}

func (s *Server) watchNodes() {
	watchList := cache.NewListWatchFromClient(
		s.clientset.CoreV1().RESTClient(),
		"nodes",
		metav1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchList,
		&v1.Node{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onNodeAdd,
			UpdateFunc: s.onNodeUpdate,
			DeleteFunc: s.onNodeDelete,
		},
	)
	controller.Run(make(chan struct{}))
}

// ipToUint32 converts an IPv4 address to a uint32
func ipToUint32(ip net.IP) uint32 {
	parts := strings.Split(ip.String(), ".")
	var result uint32
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			panic(fmt.Sprintf("Error converting IP part to integer: %s", err))
		}
		result |= uint32(num) << (24 - 8*i)
	}
	return result
}

func (s *Server) onPodAdd(obj interface{}) {
	s.handlePod(obj)
}

func (s *Server) handlePod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	ips := make([]uint32, 0, len(pod.Status.PodIPs))
	for _, podIP := range pod.Status.PodIPs {
		ip := net.ParseIP(podIP.IP)
		if ip == nil || ip.To4() == nil {
			log.Println("ip is not IPv4, currently only IPv4 is supported")
			continue
		}

		ips = append(ips, ipToUint32(ip))
	}

	s.mutex.Lock()
	s.podIndex[podKey{
		namespace: pod.Namespace,
		name:      pod.Name,
	}] = PodInfo{
		Node: pod.Spec.NodeName,
		IPs:  ips,
	}
	for _, ip := range ips {
		s.podIpIndex[ip] = podKey{
			namespace: pod.Namespace,
			name:      pod.Name,
		}
	}
	s.mutex.Unlock()
}

func (s *Server) onPodUpdate(oldObj, newObj interface{}) {
	s.handlePod(newObj)
}

type podKey struct {
	namespace string
	name      string
}

func (s *Server) onPodDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}
	k := podKey{
		namespace: pod.Namespace,
		name:      pod.Name,
	}
	s.mutex.Lock()
	info, found := s.podIndex[k]
	if found {
		for _, ip := range info.IPs {
			delete(s.podIpIndex, ip)
		}
		delete(s.podIndex, k)
	}

	delete(s.statistics, k)
	s.mutex.Unlock()
}

func (s *Server) onNodeAdd(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return
	}
	zone, ok := node.GetLabels()["topology.kubernetes.io/zone"]
	if !ok {
		zone = "unknown"
	}
	s.mutex.Lock()
	s.nodeIndex[node.Name] = zone
	s.mutex.Unlock()
}

func (s *Server) onNodeUpdate(oldObj, newObj interface{}) {
	node, ok := newObj.(*v1.Node)
	if !ok {
		return
	}
	zone, ok := node.GetLabels()["topology.kubernetes.io/zone"]
	if !ok {
		zone = "unknown"
	}
	s.mutex.Lock()
	s.nodeIndex[node.Name] = zone
	s.mutex.Unlock()
}

func (s *Server) onNodeDelete(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return
	}
	s.mutex.Lock()
	delete(s.nodeIndex, node.Name)
	s.mutex.Unlock()
	log.Printf("Node deleted: %s", node.Name)
}

type flowLog struct {
	src   podKey
	dst   podKey
	bytes int
}

func (s *Server) handlePayload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	data, err := payload.Decode(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusBadRequest)
		return
	}

	flowLogs := make([]flowLog, 0, len(data))

	s.mutex.Lock()

	for _, entry := range data {
		sourcePodKey, found := s.podIpIndex[entry.SrcIP]
		if !found {
			continue
		}

		sourcePod, found := s.podIndex[sourcePodKey]
		if !found {
			continue
		}

		srcZone, found := s.nodeIndex[sourcePod.Node]
		if !found {
			continue
		}

		dstPodKey, found := s.podIpIndex[entry.DstIP]
		if !found {
			continue
		}

		dstPod, found := s.podIndex[dstPodKey]
		if !found {
			continue
		}

		dstZone, found := s.nodeIndex[dstPod.Node]
		if !found {
			continue
		}

		if srcZone != dstZone {
			flowLogs = append(flowLogs, flowLog{
				src:   sourcePodKey,
				dst:   dstPodKey,
				bytes: int(entry.Traffic),
			})
			s.statistics[sourcePodKey] += uint64(entry.Traffic)
		}
	}

	s.mutex.Unlock()

	for _, flowLog := range flowLogs {
		log.Println(flowLog.src, "to", flowLog.dst, "with", strconv.Itoa(flowLog.bytes), "bytes")
	}
}

var (
	desc = prometheus.NewDesc(
		"pod_cross_zone_network_traffic_bytes_total",
		"The amount of cross-zone traffic the pod caused",
		[]string{"namespace", "pod"},
		nil,
	)
)

func (s *Server) Describe(ch chan<- *prometheus.Desc) {
	ch <- desc
}

func (s *Server) Collect(ch chan<- prometheus.Metric) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for pod, traffic := range s.statistics {
		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.CounterValue,
			float64(traffic),
			pod.namespace,
			pod.name,
		)
	}
}
