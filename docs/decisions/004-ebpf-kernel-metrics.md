# ADR-004: eBPF Kernel Metrics

## Status

Accepted

## Context

The /proc-based collectors provide host-level metrics, but some important signals are only available from the kernel itself:

- **TCP retransmits**: Not visible in /proc/net/dev (which only shows totals). eBPF can count retransmits per flow.
- **Syscall latency**: /proc has no syscall timing data. eBPF can build latency histograms per-syscall.
- **Block I/O latency**: /proc/diskstats shows I/O counts but not latency distributions. eBPF can measure time between request issue and completion.

Options considered:
1. **Polling /proc more aggressively** — Cannot surface the signals above.
2. **ftrace** — Requires root, text parsing, not programmatic.
3. **eBPF via cilium/ebpf** — Programmatic, efficient, widely adopted in Go.
4. **perf events** — Lower-level, less ergonomic from Go.

## Decision

Use eBPF programs attached to kernel tracepoints via the cilium/ebpf Go library. Three BPF programs:

- `tcp_retransmit.c` — Attaches to `tcp/tcp_retransmit_skb` tracepoint
- `syscall_latency.c` — Attaches to `raw_syscalls/sys_enter` and `sys_exit`
- `bio_latency.c` — Attaches to `block/block_rq_issue` and `block_rq_complete`

Each program writes to BPF maps (hash or array). The Go collector reads these maps on each collection interval and converts them to Lens metrics.

eBPF is gated behind:
- A Linux build tag (no-op on other platforms)
- A config flag `ebpf_enabled` (default false)
- Requires kernel 5.8+ and CAP_BPF or root

BPF C sources are compiled to bytecode using `bpf2go` which embeds the compiled BPF in the Go binary — no runtime compilation needed.

## Consequences

- **Linux only**: eBPF collectors only run on Linux. This is acceptable since the agent targets Linux servers.
- **Kernel version dependency**: Requires kernel 5.8+ for tracepoint attach. Documented in limitations.
- **No runtime dependency**: bpf2go embeds the compiled BPF, so the agent binary is self-contained.
- **Privileged access**: Requires CAP_BPF or root. Falls back gracefully if unavailable (logs warning, skips eBPF collectors).
- **Histogram approach**: Log2-bucketed histograms in BPF maps are space-efficient (32 slots per histogram) and allow computing percentiles in Go.
