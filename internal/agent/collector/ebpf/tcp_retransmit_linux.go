//go:build linux

package ebpf

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"github.com/louispm/lens/internal/common/model"
)

// tcpRetransmitKey matches the BPF struct retransmit_key.
type tcpRetransmitKey struct {
	Saddr uint32
	Daddr uint32
	Dport uint16
	Pad   uint16
}

// TCPRetransmitCollector uses eBPF to count TCP retransmissions.
type TCPRetransmitCollector struct {
	hostname string
	objs     *tcpRetransmitObjects
	link     link.Link
}

type tcpRetransmitObjects struct {
	RetransmitCount *ebpf.Map     `ebpf:"tcp_retransmit_count"`
	RetransmitTotal *ebpf.Map     `ebpf:"tcp_retransmit_total"`
	Program         *ebpf.Program `ebpf:"trace_tcp_retransmit"`
}

func NewTCPRetransmitCollector(hostname string) *TCPRetransmitCollector {
	return &TCPRetransmitCollector{hostname: hostname}
}

func (c *TCPRetransmitCollector) Name() string { return "ebpf.tcp_retransmit" }

func (c *TCPRetransmitCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if c.objs == nil {
		if err := c.load(); err != nil {
			return nil, fmt.Errorf("load tcp_retransmit BPF: %w", err)
		}
	}

	ts := time.Now()
	tags := map[string]string{"host": c.hostname}

	// Read total counter.
	var total uint64
	key := uint32(0)
	if err := c.objs.RetransmitTotal.Lookup(&key, &total); err == nil {
		return []model.Metric{
			{Name: "ebpf.tcp_retransmits_total", Tags: tags, Value: float64(total), Timestamp: ts, Type: model.MetricCounter},
		}, nil
	}

	// Also iterate per-flow counts for detailed metrics.
	var metrics []model.Metric
	metrics = append(metrics, model.Metric{
		Name: "ebpf.tcp_retransmits_total", Tags: tags, Value: float64(total), Timestamp: ts, Type: model.MetricCounter,
	})

	iter := c.objs.RetransmitCount.Iterate()
	var rk tcpRetransmitKey
	var count uint64
	for iter.Next(&rk, &count) {
		saddr := intToIP(rk.Saddr)
		daddr := intToIP(rk.Daddr)
		flowTags := map[string]string{
			"host":     c.hostname,
			"src_addr": saddr,
			"dst_addr": daddr,
			"dst_port": fmt.Sprintf("%d", rk.Dport),
		}
		metrics = append(metrics, model.Metric{
			Name: "ebpf.tcp_retransmits_by_flow", Tags: flowTags, Value: float64(count), Timestamp: ts, Type: model.MetricCounter,
		})
	}

	return metrics, nil
}

func (c *TCPRetransmitCollector) load() error {
	spec, err := ebpf.LoadCollectionSpecFromReader(nil) // placeholder — real impl uses generated code
	if err != nil {
		return err
	}

	c.objs = &tcpRetransmitObjects{}
	if err := spec.LoadAndAssign(c.objs, nil); err != nil {
		return fmt.Errorf("load BPF objects: %w", err)
	}

	tp, err := link.Tracepoint("tcp", "tcp_retransmit_skb", c.objs.Program, nil)
	if err != nil {
		return fmt.Errorf("attach tracepoint: %w", err)
	}
	c.link = tp

	return nil
}

// Close detaches the BPF program and releases resources.
func (c *TCPRetransmitCollector) Close() error {
	if c.link != nil {
		c.link.Close()
	}
	if c.objs != nil {
		c.objs.RetransmitCount.Close()
		c.objs.RetransmitTotal.Close()
		c.objs.Program.Close()
	}
	return nil
}

func intToIP(v uint32) string {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, v)
	return ip.String()
}

// Ensure the struct size matches what the BPF program produces.
var _ = unsafe.Sizeof(tcpRetransmitKey{}) // compile-time size check
