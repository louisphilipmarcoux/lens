package ebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPercentilesFromHist(t *testing.T) {
	tests := []struct {
		name       string
		hist       []uint64
		wantP50    float64
		wantP99    float64
	}{
		{
			name:    "empty histogram",
			hist:    make([]uint64, 32),
			wantP50: 0,
			wantP99: 0,
		},
		{
			name: "all in one bucket",
			hist: func() []uint64 {
				h := make([]uint64, 32)
				h[5] = 100 // bucket 5 = 32us
				return h
			}(),
			wantP50: 32,
			wantP99: 32,
		},
		{
			name: "spread across buckets",
			hist: func() []uint64 {
				h := make([]uint64, 32)
				h[0] = 10  // 1us
				h[3] = 40  // 8us
				h[7] = 30  // 128us
				h[10] = 15 // 1024us
				h[15] = 5  // 32768us
				return h
			}(),
			// Total = 100. p50 at count 50: 10+40=50 -> bucket 3 (8us).
			// p99 at count 99: 10+40+30+15=95, +5=100 -> bucket 15 (32768us).
			wantP50: 8,
			wantP99: 32768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p50, p99 := PercentilesFromHist(tt.hist)
			assert.Equal(t, tt.wantP50, p50, "p50")
			assert.Equal(t, tt.wantP99, p99, "p99")
		})
	}
}

func TestCollectorsReturnsEmpty(t *testing.T) {
	// On non-Linux, Collectors should return nil/empty.
	// On Linux, it would return 3 collectors (but we can't test BPF loading in unit tests).
	collectors := Collectors("testhost")
	// Just verify it doesn't panic.
	_ = collectors
}
