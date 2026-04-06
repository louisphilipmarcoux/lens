// SPDX-License-Identifier: GPL-2.0
// BPF program: Block I/O latency histogram
//
// Attaches to block_rq_issue and block_rq_complete tracepoints to measure
// the latency of block I/O requests. Results are stored in a histogram
// bucketed by log2 of latency in microseconds.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Track start time per request pointer.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u64); // request pointer
    __type(value, __u64); // start timestamp
} bio_start SEC(".maps");

#define HIST_SLOTS 32

// Histogram for read latency (log2 microseconds).
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, HIST_SLOTS);
    __type(key, __u32);
    __type(value, __u64);
} bio_read_latency_hist SEC(".maps");

// Histogram for write latency (log2 microseconds).
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, HIST_SLOTS);
    __type(key, __u32);
    __type(value, __u64);
} bio_write_latency_hist SEC(".maps");

// Total I/O request counts (index 0 = reads, index 1 = writes).
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 2);
    __type(key, __u32);
    __type(value, __u64);
} bio_count SEC(".maps");

static __always_inline __u32 log2l(__u64 v)
{
    __u32 r = 0;
    while (v > 1) {
        v >>= 1;
        r++;
    }
    return r;
}

SEC("tracepoint/block/block_rq_issue")
int block_rq_issue(struct trace_event_raw_block_rq *ctx)
{
    __u64 rq = (__u64)ctx;
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&bio_start, &rq, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/block/block_rq_complete")
int block_rq_complete(struct trace_event_raw_block_rq_completion *ctx)
{
    __u64 rq = (__u64)ctx;
    __u64 *start_ts = bpf_map_lookup_elem(&bio_start, &rq);
    if (!start_ts) {
        return 0;
    }

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ts;
    bpf_map_delete_elem(&bio_start, &rq);

    __u64 delta_us = delta_ns / 1000;
    if (delta_us == 0) delta_us = 1;

    __u32 slot = log2l(delta_us);
    if (slot >= HIST_SLOTS) slot = HIST_SLOTS - 1;

    // Determine if read or write from rwbs field.
    // rwbs[0]: 'R' for read, 'W' for write
    char rwbs = ctx->rwbs[0];
    if (rwbs == 'R' || rwbs == 'r') {
        __u64 *count = bpf_map_lookup_elem(&bio_read_latency_hist, &slot);
        if (count) __sync_fetch_and_add(count, 1);
        __u32 zero = 0;
        __u64 *total = bpf_map_lookup_elem(&bio_count, &zero);
        if (total) __sync_fetch_and_add(total, 1);
    } else {
        __u64 *count = bpf_map_lookup_elem(&bio_write_latency_hist, &slot);
        if (count) __sync_fetch_and_add(count, 1);
        __u32 one = 1;
        __u64 *total = bpf_map_lookup_elem(&bio_count, &one);
        if (total) __sync_fetch_and_add(total, 1);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
