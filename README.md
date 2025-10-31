# kubezonnet

**KUBE**rnetes cross-**ZON**e **NET**work monitoring with Prometheus for Cilium-based clusters (in Legacy host routing mode).

## Why?

While same-zone traffic is free on cloud providers, cross-zone traffic is not and can easily become a major cost factor if a lot of data is moved over the network. Therefore, understanding which workloads are causing cross-zone traffic is vital.

### Articles

Read The New Stack article: [eBPF Tool Identifies Cross-Zone Kubernetes Network Traffic](https://thenewstack.io/ebpf-tool-identifies-cross-zone-kubernetes-network-traffic/)  
Read our blog post: [kubezonnet: Monitor Cross-Zone Network Traffic in Kubernetes](https://www.polarsignals.com/blog/posts/2025/01/09/introducing-kubezonnet)

## Deploy

Kubezonnet is designed to be deployed on Kubernetes, so nothing special is required, just apply the manifests:

```bash
kubectl apply -f https://raw.githubusercontent.com/polarsignals/kubezonnet/refs/heads/main/deploy/kubezonnet.yaml
```

Container images are published at:
* agent: `ghcr.io/polarsignals/kubezonnet-agent`
* server: `ghcr.io/polarsignals/kubezonnet-server`

## Requirements

* Cilium as the CNI (in Legacy host routing mode, otherwise netfilter won't work correctly, GKE dataplane v2 clusters use this mode)
* Linux Kernel 6.4+ (netfilter eBPF programs were only added in 6.4)

## How does it work?

Kubezonnet is made up of two components:

* kubezonnet-agent: collects traffic statistics, using eBPF with a netfilter postrouting hook, about all Pod network traffic and sends the statistics to the server. This component is
 deployed on all nodes. It aggregates the statistics per source and destination IP and sends them to the server every 10 seconds.
* kubezonnet-server: aggregates the statistics sent from the agents and resolves the actual pod, node and zone relationships of the network statistics, and then exposes the statistic
s on a Prometheus metrics endpoint. This component can either be deployed once per cluster, or for each zone (once again to save cross-zone traffic).

## How do I use it?

### Metrics

The server portion of kubezonnet exposes a Prometheus metrics endpoint on port 8080, which can be scraped by Prometheus. Once set up the `pod_cross_zone_network_traffic_bytes_total`
counter will be available.

This will show the top 20 pods by cross-zone network traffic per second in the last 5 minutes, in megabytes.

```promql
topk(20, rate(pod_cross_zone_network_traffic_bytes_total[5m])) / 1e6
```

When trying to understand a cloud bill, the cumulative amount over a timeframe may be more interesting than the current usage. This query will show the top 20 pods by cross-zone netw
ork traffic in the last week, in gigabytes:

```promql
topk(20, increase(pod_cross_zone_network_traffic_bytes_total[1w])) / 1e9
```

### Logs

The server also logs something akin to flow logs, which can be used to understand the network traffic in more detail. They print the source and destination pods in addition to the ne
twork traffic associated whenever agents send statistics (every 10 seconds).

## Limitations

* Currently only supports IPv4.
* Traffic statistics use the IP packet sizes, therefore skip the IP header part. It's recommended to use these statistics to understand ratios of traffic and not use it for metering purposes or comparing them to other lower level network statistics that include the IP header.

## Roadmap

* Support for IPv6.
* Sum metrics by workload (deployment, statefulset, etc.), since pod granularity is not necessary to get the same insights and when higher granularity is needed, the logs can be used
.

## Acknowledgments

Various people have helped in the process of putting some of the pieces of this project together. In no particular order that includes, but is not limited to:

* [Dylan Reimerink](https://github.com/dylandreimerink)
* [Casey Callendrello](https://github.com/squeed)
* [Chance Zibolski](https://github.com/chancez)
* [Florian Lehner](https://github.com/florianl)
