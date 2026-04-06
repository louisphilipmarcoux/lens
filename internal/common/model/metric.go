package model

import "time"

// MetricType represents the type of a metric.
type MetricType int

const (
	MetricGauge MetricType = iota
	MetricCounter
	MetricHistogram
)

func (t MetricType) String() string {
	switch t {
	case MetricGauge:
		return "gauge"
	case MetricCounter:
		return "counter"
	case MetricHistogram:
		return "histogram"
	default:
		return "unknown"
	}
}

// Metric represents a single metric data point.
type Metric struct {
	Name      string            `json:"name"`
	Tags      map[string]string `json:"tags"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Type      MetricType        `json:"type"`
}
