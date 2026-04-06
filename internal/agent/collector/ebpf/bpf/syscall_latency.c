// SPDX-License-Identifier: GPL-2.0
// BPF program: Syscall latency histogram
//
// Attaches to raw_syscalls:sys_enter and sys_exit tracepoints to measure
// the latency of each syscall. Results are stored in a histogram
// bucketed by log2 of latency in microseconds.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Per-CPU map to track syscall entry timestamps.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32); // pid_tgid (tid)
    __type(value, __u64); // entry timestamp
} syscall_start SEC(".maps");

// Histogram: slot = log2(latency_us), value = count.
// 32 slots covers 1us to ~4 billion us (~4000 seconds).
#define HIST_SLOTS 32

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, HIST_SLOTS);
    __type(key, __u32);
    __type(value, __u64);
} syscall_latency_hist SEC(".maps");

// Total syscall count.
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} syscall_count SEC(".maps");

static __always_inline __u32 log2l(__u64 v)
{
    __u32 r = 0;
    while (v > 1) {
        v >>= 1;
        r++;
    }
    return r;
}

SEC("tracepoint/raw_syscalls/sys_enter")
int sys_enter(struct trace_event_raw_sys_enter *ctx)
{
    __u32 tid = (__u32)bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&syscall_start, &tid, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/raw_syscalls/sys_exit")
int sys_exit(struct trace_event_raw_sys_exit *ctx)
{
    __u32 tid = (__u32)bpf_get_current_pid_tgid();
    __u64 *start_ts = bpf_map_lookup_elem(&syscall_start, &tid);
    if (!start_ts) {
        return 0;
    }

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ts;
    bpf_map_delete_elem(&syscall_start, &tid);

    // Convert to microseconds and bucket into log2 histogram.
    __u64 delta_us = delta_ns / 1000;
    if (delta_us == 0) delta_us = 1;

    __u32 slot = log2l(delta_us);
    if (slot >= HIST_SLOTS) slot = HIST_SLOTS - 1;

    __u64 *count = bpf_map_lookup_elem(&syscall_latency_hist, &slot);
    if (count) {
        __sync_fetch_and_add(count, 1);
    }

    // Increment total.
    __u32 zero = 0;
    __u64 *total = bpf_map_lookup_elem(&syscall_count, &zero);
    if (total) {
        __sync_fetch_and_add(total, 1);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
