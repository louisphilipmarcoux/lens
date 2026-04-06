//go:build linux

package ebpf

import (
	"context"
	"fmt"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"github.com/louispm/lens/internal/common/model"
)

const histSlots = 32

// SyscallLatencyCollector uses eBPF to measure syscall latency distributions.
type SyscallLatencyCollector struct {
	hostname string
	objs     *syscallLatencyObjects
	links    []link.Link
}

type syscallLatencyObjects struct {
	LatencyHist  *ebpf.Map     `ebpf:"syscall_latency_hist"`
	SyscallCount *ebpf.Map     `ebpf:"syscall_count"`
	SysEnter     *ebpf.Program `ebpf:"sys_enter"`
	SysExit      *ebpf.Program `ebpf:"sys_exit"`
}

func NewSyscallLatencyCollector(hostname string) *SyscallLatencyCollector {
	return &SyscallLatencyCollector{hostname: hostname}
}

func (c *SyscallLatencyCollector) Name() string { return "ebpf.syscall_latency" }

func (c *SyscallLatencyCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if c.objs == nil {
		if err := c.load(); err != nil {
			return nil, fmt.Errorf("load syscall_latency BPF: %w", err)
		}
	}

	ts := time.Now()
	tags := map[string]string{"host": c.hostname}
	var metrics []model.Metric

	// Read total syscall count.
	var total uint64
	key := uint32(0)
	if err := c.objs.SyscallCount.Lookup(&key, &total); err == nil {
		metrics = append(metrics, model.Metric{
			Name: "ebpf.syscalls_total", Tags: tags, Value: float64(total), Timestamp: ts, Type: model.MetricCounter,
		})
	}

	// Read histogram and compute percentiles.
	hist := make([]uint64, histSlots)
	for i := uint32(0); i < histSlots; i++ {
		var count uint64
		if err := c.objs.LatencyHist.Lookup(&i, &count); err == nil {
			hist[i] = count
		}
	}

	p50, p99 := PercentilesFromHist(hist)
	metrics = append(metrics,
		model.Metric{Name: "ebpf.syscall_latency_p50_us", Tags: tags, Value: p50, Timestamp: ts, Type: model.MetricGauge},
		model.Metric{Name: "ebpf.syscall_latency_p99_us", Tags: tags, Value: p99, Timestamp: ts, Type: model.MetricGauge},
	)

	// Emit histogram buckets as individual metrics.
	for i, count := range hist {
		if count == 0 {
			continue
		}
		bucketTags := map[string]string{
			"host":         c.hostname,
			"bucket_le_us": fmt.Sprintf("%d", 1<<uint(i)),
		}
		metrics = append(metrics, model.Metric{
			Name: "ebpf.syscall_latency_bucket", Tags: bucketTags, Value: float64(count), Timestamp: ts, Type: model.MetricCounter,
		})
	}

	return metrics, nil
}

func (c *SyscallLatencyCollector) load() error {
	spec, err := ebpf.LoadCollectionSpecFromReader(nil) // placeholder
	if err != nil {
		return err
	}

	c.objs = &syscallLatencyObjects{}
	if err := spec.LoadAndAssign(c.objs, nil); err != nil {
		return fmt.Errorf("load BPF objects: %w", err)
	}

	enterLink, err := link.Tracepoint("raw_syscalls", "sys_enter", c.objs.SysEnter, nil)
	if err != nil {
		return fmt.Errorf("attach sys_enter: %w", err)
	}
	exitLink, err := link.Tracepoint("raw_syscalls", "sys_exit", c.objs.SysExit, nil)
	if err != nil {
		enterLink.Close()
		return fmt.Errorf("attach sys_exit: %w", err)
	}
	c.links = []link.Link{enterLink, exitLink}

	return nil
}

// Close detaches BPF programs and releases resources.
func (c *SyscallLatencyCollector) Close() error {
	for _, l := range c.links {
		l.Close()
	}
	if c.objs != nil {
		c.objs.LatencyHist.Close()
		c.objs.SyscallCount.Close()
		c.objs.SysEnter.Close()
		c.objs.SysExit.Close()
	}
	return nil
}

