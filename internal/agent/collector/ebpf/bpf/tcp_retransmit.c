// SPDX-License-Identifier: GPL-2.0
// BPF program: TCP retransmit counter
//
// Attaches to the tcp_retransmit_skb tracepoint and counts retransmits
// per source IP + destination IP + destination port.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_endian.h>

struct retransmit_key {
    __u32 saddr;
    __u32 daddr;
    __u16 dport;
    __u16 pad;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, struct retransmit_key);
    __type(value, __u64);
} tcp_retransmit_count SEC(".maps");

// Total retransmit counter (single entry).
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} tcp_retransmit_total SEC(".maps");

SEC("tracepoint/tcp/tcp_retransmit_skb")
int trace_tcp_retransmit(struct trace_event_raw_tcp_event_sk_skb *ctx)
{
    struct retransmit_key key = {};
    key.saddr = ctx->saddr;
    key.daddr = ctx->daddr;
    key.dport = ctx->dport;

    __u64 *count = bpf_map_lookup_elem(&tcp_retransmit_count, &key);
    if (count) {
        __sync_fetch_and_add(count, 1);
    } else {
        __u64 init = 1;
        bpf_map_update_elem(&tcp_retransmit_count, &key, &init, BPF_ANY);
    }

    // Increment total counter.
    __u32 zero = 0;
    __u64 *total = bpf_map_lookup_elem(&tcp_retransmit_total, &zero);
    if (total) {
        __sync_fetch_and_add(total, 1);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
