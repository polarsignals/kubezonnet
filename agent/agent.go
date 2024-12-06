package agent

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/polarsignals/kubezonnet/byteorder"
	"github.com/polarsignals/kubezonnet/payload"
)

func Run(node, subnetCidr, server string, flushInterval time.Duration, debug, sendData bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	fmt.Println("Watching pods for node: ", node)
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = "spec.nodeName=" + node
		}))
	informer := factory.Core().V1().Pods().Informer()
	go informer.Run(ctx.Done())

	ip, ipNet, err := net.ParseCIDR(subnetCidr)
	if err != nil || ip == nil || ip.To4() == nil {
		fmt.Println("Error: Invalid subnet CIDR")
		flag.Usage()
		os.Exit(1)
	}

	// Convert IP to uint32
	ipUint := ipToUint32(ip)

	// Convert subnet mask to uint32
	maskUint := maskToUint32(ipNet.Mask)

	// Print the converted values
	fmt.Printf("Subnet CIDR: %s\n", subnetCidr)
	fmt.Printf("IP Prefix: %s -> %x\n", ip.String(), ipUint)
	fmt.Printf("Subnet Mask: %s -> %x\n", net.IP(ipNet.Mask).String(), maskUint)

	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("removing memlock: %w", err)
	}

	// Load the compiled eBPF ELF and load it into the kernel.
	spec, err := loadKubezonnet()
	if err != nil {
		return fmt.Errorf("load eBPF program: %w", err)
	}

	if err := spec.RewriteConstants(map[string]interface{}{
		"subnet_prefix": byteorder.Htonl(ipUint),
		"subnet_mask":   byteorder.Htonl(maskUint),
	}); err != nil {
		return fmt.Errorf("configure eBPF program: %w", err)
	}

	var objs kubezonnetObjects
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return fmt.Errorf("load eBPF objects: %w", err)
	}
	defer objs.Close()

	link, err := link.AttachNetfilter(link.NetfilterOptions{
		ProtocolFamily: 2, // IPv4
		HookNumber:     4, // netfilter postrouting
		Program:        objs.NfPostroutingHook,
	})
	if err != nil {
		log.Printf("attach netfilter: %q", err)
	}
	defer link.Close()

	// Channel to listen to interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	size := objs.IpMap.MaxEntries()
	keys := make([]payload.IPKey, size)
	values := make([]payload.IPValue, size)
	for {
		select {
		case <-stop:
			fmt.Println("Shutting down...")
			return nil
		case <-ctx.Done():
			fmt.Println("Shutting down...")
			return nil
		case <-ticker.C:
			log.Println("reading data from eBPF maps")
			keys = keys[:size]
			values = values[:size]
			opts := &ebpf.BatchOptions{}
			cursor := new(ebpf.MapBatchCursor)
			n, err := objs.IpMap.BatchLookupAndDelete(cursor, keys, values, opts)
			if n <= 0 {
				log.Println("no data, skipping")
				continue
			}
			if err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
				log.Println("failed to read data:", err)
				continue
			}
			keys = keys[:n]
			values = values[:n]

			pods := convertToPods(informer.GetStore().List())
			finalKeys, finalValues := filterSrcIpOnCurrentHost(keys, values, pods)

			if debug {
				log.Println("debug printing", len(finalKeys), "keys, started with", n, "keys before filtering to host-local pods (", len(pods), ")")
				for i := 0; i < len(finalKeys); i++ {
					srcIP := net.IPv4(byte(finalKeys[i].SrcIP), byte(finalKeys[i].SrcIP>>8), byte(finalKeys[i].SrcIP>>16), byte(finalKeys[i].SrcIP>>24)).String()
					dstIP := net.IPv4(byte(finalKeys[i].DstIP), byte(finalKeys[i].DstIP>>8), byte(finalKeys[i].DstIP>>16), byte(finalKeys[i].DstIP>>24)).String()
					fmt.Printf("%s -> %s: %d bytes\n", srcIP, dstIP, finalValues[i].PacketSize)
				}
			}

			if sendData {
				if len(finalKeys) > 0 {
					log.Println("sending data to the server")
					if err := sendDataToServer(ctx, server, finalKeys, finalValues); err != nil {
						log.Println(err)
					}
				}
			} else {
				log.Println("sending data disabled, skipping")
			}
		}
	}
}

func convertToPods(objs []interface{}) []*v1.Pod {
	res := make([]*v1.Pod, 0, len(objs))

	for _, obj := range objs {
		res = append(res, obj.(*v1.Pod))
	}

	return res
}

func filterSrcIpOnCurrentHost(keys []payload.IPKey, values []payload.IPValue, podsOnHost []*v1.Pod) ([]payload.IPKey, []payload.IPValue) {
	ipsOnHost := make(map[uint32]struct{}, len(podsOnHost)) // pods may have multiple IPs so this is just an approximation

	for _, pod := range podsOnHost {
		for _, podIP := range pod.Status.PodIPs {
			ip := net.ParseIP(podIP.IP)
			if ip == nil || ip.To4() == nil {
				log.Println("ip is not IPv4, currently only IPv4 is supported")
				continue
			}

			ipsOnHost[ipToUint32(ip)] = struct{}{}
		}
	}

	resKeys := make([]payload.IPKey, 0, len(keys))
	resValues := make([]payload.IPValue, 0, len(values))
	for i := range keys {
		if _, found := ipsOnHost[byteorder.Ntohl(keys[i].SrcIP)]; found {
			resKeys = append(resKeys, keys[i])
			resValues = append(resValues, values[i])
		}
	}

	return resKeys, resValues
}

func attachToInterface(interfaceName string, prog *ebpf.Program) (link.Link, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("get interface %q: %w", interfaceName, err)
	}

	link, err := link.AttachTCX(link.TCXOptions{
		Program:   prog,
		Attach:    ebpf.AttachTCXEgress,
		Interface: iface.Index,
	})
	if err != nil {
		return nil, fmt.Errorf("attach TCX: %w", err)
	}

	return link, err
}

func sendDataToServer(ctx context.Context, server string, keys []payload.IPKey, values []payload.IPValue) error {
	content := payload.Encode(keys, values)
	req, err := http.NewRequest("POST", server, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req = req.WithContext(ctx)

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	respContent, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("write not successful: %s", string(respContent))
	}

	return nil
}

type PacketInfo struct {
	SrcIP      uint32
	DestIP     uint32
	PacketSize uint32
	Ifindex    uint32
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

// maskToUint32 converts a net.IPMask to a uint32
func maskToUint32(mask net.IPMask) uint32 {
	var result uint32
	for _, byteValue := range mask {
		result = (result << 8) | uint32(byteValue)
	}
	return result
}
