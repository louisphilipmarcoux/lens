package model

// Batch represents a collection of data points to be shipped together.
type Batch struct {
	Metrics []Metric   `json:"metrics,omitempty"`
	Logs    []LogEntry `json:"logs,omitempty"`
	Spans   []Span     `json:"spans,omitempty"`
}
