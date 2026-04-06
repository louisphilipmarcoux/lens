package model

import "time"

// SpanStatus represents the outcome of a span.
type SpanStatus int

const (
	SpanStatusUnset SpanStatus = iota
	SpanStatusOK
	SpanStatusError
)

// SpanEvent represents a timestamped annotation on a span.
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Span represents a single distributed trace span.
type Span struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Service   string            `json:"service"`
	Operation string            `json:"operation"`
	StartTime time.Time         `json:"start_time"`
	Duration  time.Duration     `json:"duration"`
	Status    SpanStatus        `json:"status"`
	Tags      map[string]string `json:"tags,omitempty"`
	Events    []SpanEvent       `json:"events,omitempty"`
}
