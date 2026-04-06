//go:build linux

package ebpf

import (
	"github.com/louispm/lens/internal/agent/collector/proc"
)

func platformCollectors(hostname string) []proc.Collector {
	return []proc.Collector{
		NewTCPRetransmitCollector(hostname),
		NewSyscallLatencyCollector(hostname),
		NewBioLatencyCollector(hostname),
	}
}
