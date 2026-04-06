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

// BioLatencyCollector uses eBPF to measure block I/O latency distributions.
type BioLatencyCollector struct {
	hostname string
	objs     *bioLatencyObjects
	links    []link.Link
}

type bioLatencyObjects struct {
	ReadLatencyHist  *ebpf.Map     `ebpf:"bio_read_latency_hist"`
	WriteLatencyHist *ebpf.Map     `ebpf:"bio_write_latency_hist"`
	BioCount         *ebpf.Map     `ebpf:"bio_count"`
	BlockRqIssue     *ebpf.Program `ebpf:"block_rq_issue"`
	BlockRqComplete  *ebpf.Program `ebpf:"block_rq_complete"`
}

func NewBioLatencyCollector(hostname string) *BioLatencyCollector {
	return &BioLatencyCollector{hostname: hostname}
}

func (c *BioLatencyCollector) Name() string { return "ebpf.bio_latency" }

func (c *BioLatencyCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if c.objs == nil {
		if err := c.load(); err != nil {
			return nil, fmt.Errorf("load bio_latency BPF: %w", err)
		}
	}

	ts := time.Now()
	tags := map[string]string{"host": c.hostname}
	var metrics []model.Metric

	// Read total I/O counts (index 0 = reads, 1 = writes).
	var readTotal, writeTotal uint64
	readKey, writeKey := uint32(0), uint32(1)
	c.objs.BioCount.Lookup(&readKey, &readTotal)
	c.objs.BioCount.Lookup(&writeKey, &writeTotal)

	metrics = append(metrics,
		model.Metric{Name: "ebpf.bio_reads_total", Tags: tags, Value: float64(readTotal), Timestamp: ts, Type: model.MetricCounter},
		model.Metric{Name: "ebpf.bio_writes_total", Tags: tags, Value: float64(writeTotal), Timestamp: ts, Type: model.MetricCounter},
	)

	// Read latency histograms and compute percentiles.
	readHist := c.readHist(c.objs.ReadLatencyHist)
	writeHist := c.readHist(c.objs.WriteLatencyHist)

	readP50, readP99 := PercentilesFromHist(readHist)
	writeP50, writeP99 := PercentilesFromHist(writeHist)

	metrics = append(metrics,
		model.Metric{Name: "ebpf.bio_read_latency_p50_us", Tags: tags, Value: readP50, Timestamp: ts, Type: model.MetricGauge},
		model.Metric{Name: "ebpf.bio_read_latency_p99_us", Tags: tags, Value: readP99, Timestamp: ts, Type: model.MetricGauge},
		model.Metric{Name: "ebpf.bio_write_latency_p50_us", Tags: tags, Value: writeP50, Timestamp: ts, Type: model.MetricGauge},
		model.Metric{Name: "ebpf.bio_write_latency_p99_us", Tags: tags, Value: writeP99, Timestamp: ts, Type: model.MetricGauge},
	)

	// Emit histogram buckets.
	for _, pair := range []struct {
		name string
		hist []uint64
	}{
		{"ebpf.bio_read_latency_bucket", readHist},
		{"ebpf.bio_write_latency_bucket", writeHist},
	} {
		for i, count := range pair.hist {
			if count == 0 {
				continue
			}
			bucketTags := map[string]string{
				"host":         c.hostname,
				"bucket_le_us": fmt.Sprintf("%d", 1<<uint(i)),
			}
			metrics = append(metrics, model.Metric{
				Name: pair.name, Tags: bucketTags, Value: float64(count), Timestamp: ts, Type: model.MetricCounter,
			})
		}
	}

	return metrics, nil
}

func (c *BioLatencyCollector) readHist(m *ebpf.Map) []uint64 {
	hist := make([]uint64, histSlots)
	for i := uint32(0); i < histSlots; i++ {
		var count uint64
		if err := m.Lookup(&i, &count); err == nil {
			hist[i] = count
		}
	}
	return hist
}

func (c *BioLatencyCollector) load() error {
	spec, err := ebpf.LoadCollectionSpecFromReader(nil) // placeholder
	if err != nil {
		return err
	}

	c.objs = &bioLatencyObjects{}
	if err := spec.LoadAndAssign(c.objs, nil); err != nil {
		return fmt.Errorf("load BPF objects: %w", err)
	}

	issueLink, err := link.Tracepoint("block", "block_rq_issue", c.objs.BlockRqIssue, nil)
	if err != nil {
		return fmt.Errorf("attach block_rq_issue: %w", err)
	}
	completeLink, err := link.Tracepoint("block", "block_rq_complete", c.objs.BlockRqComplete, nil)
	if err != nil {
		issueLink.Close()
		return fmt.Errorf("attach block_rq_complete: %w", err)
	}
	c.links = []link.Link{issueLink, completeLink}

	return nil
}

// Close detaches BPF programs and releases resources.
func (c *BioLatencyCollector) Close() error {
	for _, l := range c.links {
		l.Close()
	}
	if c.objs != nil {
		c.objs.ReadLatencyHist.Close()
		c.objs.WriteLatencyHist.Close()
		c.objs.BioCount.Close()
		c.objs.BlockRqIssue.Close()
		c.objs.BlockRqComplete.Close()
	}
	return nil
}
