package model

import "time"

// LogEntry represents a single structured log record.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Service   string            `json:"service"`
	Host      string            `json:"host"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Fields    map[string]any    `json:"fields,omitempty"`
	Source    string            `json:"source"`
}
