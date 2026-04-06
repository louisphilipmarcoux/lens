package ebpf

import "math"

// PercentilesFromHist computes p50 and p99 from a log2-bucketed histogram.
// Exported for testing. Each slot i represents values in [2^i, 2^(i+1)).
func PercentilesFromHist(hist []uint64) (p50, p99 float64) {
	var total uint64
	for _, c := range hist {
		total += c
	}
	if total == 0 {
		return 0, 0
	}

	p50Threshold := uint64(math.Ceil(float64(total) * 0.50))
	p99Threshold := uint64(math.Ceil(float64(total) * 0.99))

	var cumulative uint64
	for i, c := range hist {
		cumulative += c
		bucketMid := float64(uint64(1) << uint(i))
		if p50 == 0 && cumulative >= p50Threshold {
			p50 = bucketMid
		}
		if p99 == 0 && cumulative >= p99Threshold {
			p99 = bucketMid
			break
		}
	}

	return p50, p99
}
