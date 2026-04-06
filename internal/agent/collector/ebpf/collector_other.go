//go:build !linux

package ebpf

import (
	"github.com/louispm/lens/internal/agent/collector/proc"
)

func platformCollectors(_ string) []proc.Collector {
	return nil
}
