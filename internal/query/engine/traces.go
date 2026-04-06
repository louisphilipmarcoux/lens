package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TraceSpan represents a span in a trace query result.
type TraceSpan struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id"`
	Service   string            `json:"service"`
	Operation string            `json:"operation"`
	StartTime time.Time         `json:"start_time"`
	Duration  time.Duration     `json:"duration"`
	Status    string            `json:"status"`
	Tags      map[string]string `json:"tags"`
	Events    json.RawMessage   `json:"events"`
	Children  []*TraceSpan      `json:"children,omitempty"`
}

// TraceResult is the complete trace with its span tree.
type TraceResult struct {
	TraceID  string      `json:"trace_id"`
	Root     *TraceSpan  `json:"root"`
	Spans    []TraceSpan `json:"spans"`
	Duration time.Duration `json:"duration"`
	Services []string    `json:"services"`
}

// TraceSearchQuery represents a trace search request.
type TraceSearchQuery struct {
	Service     string
	Operation   string
	MinDuration time.Duration
	MaxDuration time.Duration
	Tags        map[string]string
	Start       time.Time
	End         time.Time
	Limit       int
}

// TraceEngine executes trace queries against ClickHouse.
type TraceEngine struct {
	db *sql.DB
}

// NewTraceEngine creates a trace query engine.
func NewTraceEngine(db *sql.DB) *TraceEngine {
	return &TraceEngine{db: db}
}

// GetTrace returns a complete trace by ID, assembled into a span tree.
func (e *TraceEngine) GetTrace(ctx context.Context, traceID string) (*TraceResult, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT trace_id, span_id, parent_id, service, operation,
		       start_time, duration_ns, status, toString(tags), events
		FROM traces FINAL
		WHERE trace_id = $1
		ORDER BY start_time ASC
	`, traceID)
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}
	defer rows.Close()

	var spans []TraceSpan
	serviceSet := make(map[string]bool)

	for rows.Next() {
		var s TraceSpan
		var durationNs uint64
		var tagsStr string
		var eventsStr string

		if err := rows.Scan(&s.TraceID, &s.SpanID, &s.ParentID, &s.Service,
			&s.Operation, &s.StartTime, &durationNs, &s.Status, &tagsStr, &eventsStr); err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
		}

		s.Duration = time.Duration(durationNs)
		s.Tags = parseTags(tagsStr)
		s.Events = json.RawMessage(eventsStr)
		serviceSet[s.Service] = true
		spans = append(spans, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(spans) == 0 {
		return nil, nil
	}

	// Build span tree.
	root := buildSpanTree(spans)

	services := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		services = append(services, s)
	}

	// Total trace duration.
	var totalDuration time.Duration
	if root != nil {
		totalDuration = root.Duration
	}

	return &TraceResult{
		TraceID:  traceID,
		Root:     root,
		Spans:    spans,
		Duration: totalDuration,
		Services: services,
	}, nil
}

// SearchTraces finds traces matching the given criteria.
func (e *TraceEngine) SearchTraces(ctx context.Context, q *TraceSearchQuery) ([]TraceResult, error) {
	whereClause, args := e.buildSearchWhere(q)

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Find distinct trace IDs matching criteria.
	querySQL := fmt.Sprintf(`
		SELECT DISTINCT trace_id
		FROM traces FINAL
		WHERE %s
		ORDER BY min(start_time) DESC
		LIMIT %d
	`, whereClause, limit)

	rows, err := e.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("search traces: %w", err)
	}
	defer rows.Close()

	var traceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		traceIDs = append(traceIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch full traces.
	var results []TraceResult
	for _, id := range traceIDs {
		trace, err := e.GetTrace(ctx, id)
		if err != nil {
			return nil, err
		}
		if trace != nil {
			results = append(results, *trace)
		}
	}

	return results, nil
}

func (e *TraceEngine) buildSearchWhere(q *TraceSearchQuery) (string, []any) {
	var where []string
	var args []any
	idx := 0
	nextArg := func(v any) string {
		idx++
		args = append(args, v)
		return fmt.Sprintf("$%d", idx)
	}

	if !q.Start.IsZero() {
		where = append(where, fmt.Sprintf("start_time >= %s", nextArg(q.Start)))
	}
	if !q.End.IsZero() {
		where = append(where, fmt.Sprintf("start_time <= %s", nextArg(q.End)))
	}
	if q.Service != "" {
		where = append(where, fmt.Sprintf("service = %s", nextArg(q.Service)))
	}
	if q.Operation != "" {
		where = append(where, fmt.Sprintf("operation = %s", nextArg(q.Operation)))
	}
	if q.MinDuration > 0 {
		where = append(where, fmt.Sprintf("duration_ns >= %s", nextArg(uint64(q.MinDuration.Nanoseconds()))))
	}
	if q.MaxDuration > 0 {
		where = append(where, fmt.Sprintf("duration_ns <= %s", nextArg(uint64(q.MaxDuration.Nanoseconds()))))
	}
	for k, v := range q.Tags {
		where = append(where, fmt.Sprintf("tags[%s] = %s", nextArg(k), nextArg(v)))
	}

	if len(where) == 0 {
		return "1 = 1", nil
	}
	return strings.Join(where, " AND "), args
}

// buildSpanTree assembles a flat list of spans into a tree rooted at the span
// with no parent (or the first span if no root is found).
func buildSpanTree(spans []TraceSpan) *TraceSpan {
	spanMap := make(map[string]*TraceSpan, len(spans))
	for i := range spans {
		spanMap[spans[i].SpanID] = &spans[i]
	}

	var root *TraceSpan
	for i := range spans {
		s := &spans[i]
		if s.ParentID == "" || s.ParentID == "00000000000000000000000000000000" {
			root = s
		} else if parent, ok := spanMap[s.ParentID]; ok {
			parent.Children = append(parent.Children, s)
		}
	}

	if root == nil && len(spans) > 0 {
		root = &spans[0]
	}

	return root
}
