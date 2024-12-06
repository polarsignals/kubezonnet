//go:build ignore
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define NF_DROP         0
#define NF_ACCEPT       1
#define ETH_P_IP        0x0800
#define ETH_P_IPV6      0x86DD
#define IP_MF           0x2000
#define IP_OFFSET       0x1FFF
#define NEXTHDR_FRAGMENT    44

extern int bpf_dynptr_from_skb(struct __sk_buff *skb, __u64 flags,
                  struct bpf_dynptr *ptr__uninit) __ksym;
extern void *bpf_dynptr_slice(const struct bpf_dynptr *ptr, uint32_t offset,
                  void *buffer, uint32_t buffer__sz) __ksym;

struct ip_key {
    __u32 src_ip;
    __u32 dest_ip;
};

struct ip_value {
    __u64 packet_size;
};

// Map to store cumulative packet sizes for each source-destination pair
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct ip_key);
    __type(value, struct ip_value);
    __uint(max_entries, 1024);
} ip_map SEC(".maps");

volatile const __u32 subnet_prefix;
volatile const __u32 subnet_mask;

static int handle_v4(struct __sk_buff *skb)
{
    struct bpf_dynptr ptr;
    u8 iph_buf[20] = {};
    struct iphdr *ip;

    if (bpf_dynptr_from_skb(skb, 0, &ptr))
        return NF_ACCEPT;

    ip = bpf_dynptr_slice(&ptr, 0, iph_buf, sizeof(iph_buf));
    if (!ip)
        return NF_ACCEPT;

    if ((ip->saddr & subnet_mask) == subnet_prefix && (ip->daddr & subnet_mask) == subnet_prefix) {
        struct ip_key key = {};
        key.src_ip = ip->saddr;
        key.dest_ip = ip->daddr;

        __u64 packet_size = (__u64)(ip->tot_len);

        // Lookup or initialize the value in the map
        struct ip_value *value = bpf_map_lookup_elem(&ip_map, &key);
        if (value) {
            // Increment the packet size
            __sync_fetch_and_add(&value->packet_size, packet_size);
        } else {
            // Initialize a new entry
            struct ip_value new_value = {};
            new_value.packet_size = packet_size;
            bpf_map_update_elem(&ip_map, &key, &new_value, BPF_ANY);
        }
    }

    return NF_ACCEPT;
}

SEC("netfilter/postrouting")
int nf_postrouting_hook(struct bpf_nf_ctx *ctx) {
    struct __sk_buff *skb = (struct __sk_buff *)ctx->skb;

    switch (bpf_ntohs(ctx->skb->protocol)) {
        case ETH_P_IP:
            return handle_v4(skb);
        case ETH_P_IPV6:
            return NF_ACCEPT; // don't support IPv6 yet
        default:
            return NF_ACCEPT;
    }

    return NF_ACCEPT;
}

char __license[] SEC("license") = "Dual MIT/GPL";
