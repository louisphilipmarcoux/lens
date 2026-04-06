// Package ebpf provides eBPF-based kernel metric collectors.
//
// eBPF programs attach to kernel tracepoints and kprobes to collect
// metrics that are not available via /proc, including:
//   - TCP retransmit counts per source/destination
//   - Syscall latency histograms
//   - Block I/O latency histograms
//
// This package requires Linux with kernel 5.8+ and CAP_BPF or root.
// On non-Linux platforms, a no-op implementation is provided.
package ebpf

import (
	"github.com/louispm/lens/internal/agent/collector/proc"
)

// Collectors returns all eBPF-based collectors for the current platform.
// On non-Linux platforms, this returns an empty slice.
func Collectors(hostname string) []proc.Collector {
	return platformCollectors(hostname)
}
