// Package engine implements the query engines for metrics, logs, and traces.
package engine

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MetricSample represents a single time-series data point.
type MetricSample struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
}

// MetricResult is the result of a metric query.
type MetricResult struct {
	Name    string         `json:"name"`
	Samples []MetricSample `json:"samples"`
}

// PromQLQuery represents a parsed PromQL-compatible query.
type PromQLQuery struct {
	MetricName string
	Filters    map[string]string
	Function   string // avg, sum, max, min, count, rate
	GroupBy    []string
	Start      time.Time
	End        time.Time
	Step       time.Duration
}

// MetricEngine executes metric queries against ClickHouse.
type MetricEngine struct {
	db *sql.DB
}

// NewMetricEngine creates a metric query engine.
func NewMetricEngine(db *sql.DB) *MetricEngine {
	return &MetricEngine{db: db}
}

// Query executes a PromQL-compatible query.
func (e *MetricEngine) Query(ctx context.Context, q *PromQLQuery) ([]MetricResult, error) {
	sqlQuery, args := e.buildSQL(q)

	rows, err := e.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	resultMap := make(map[string]*MetricResult)

	for rows.Next() {
		var ts time.Time
		var value float64
		var name string
		var tagsStr string

		if err := rows.Scan(&ts, &value, &name, &tagsStr); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		key := name + "|" + tagsStr
		result, ok := resultMap[key]
		if !ok {
			result = &MetricResult{Name: name}
			resultMap[key] = result
		}
		result.Samples = append(result.Samples, MetricSample{
			Timestamp: ts,
			Value:     value,
			Tags:      parseTags(tagsStr),
		})
	}

	results := make([]MetricResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}
	return results, rows.Err()
}

// InstantQuery executes a query at a single point in time.
func (e *MetricEngine) InstantQuery(ctx context.Context, metricName string, filters map[string]string, at time.Time) ([]MetricSample, error) {
	q := &PromQLQuery{
		MetricName: metricName,
		Filters:    filters,
		Start:      at.Add(-5 * time.Minute),
		End:        at,
	}
	sqlQuery, args := e.buildInstantSQL(q)
	rows, err := e.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("instant query: %w", err)
	}
	defer rows.Close()

	var samples []MetricSample
	for rows.Next() {
		var value float64
		var tagsStr string
		if err := rows.Scan(&value, &tagsStr); err != nil {
			return nil, err
		}
		samples = append(samples, MetricSample{
			Timestamp: at,
			Value:     value,
			Tags:      parseTags(tagsStr),
		})
	}
	return samples, rows.Err()
}

func (e *MetricEngine) buildSQL(q *PromQLQuery) (string, []any) {
	var args []any
	argIdx := 0

	nextArg := func(v any) string {
		argIdx++
		args = append(args, v)
		return fmt.Sprintf("$%d", argIdx)
	}

	aggFunc := "avg"
	if q.Function != "" {
		switch q.Function {
		case "sum", "avg", "min", "max", "count":
			aggFunc = q.Function
		case "rate":
			aggFunc = "max" // rate computed in post-processing
		}
	}

	// Build time bucket.
	step := q.Step
	if step == 0 {
		step = 60 * time.Second
	}
	stepSeconds := int(step.Seconds())

	var where []string
	where = append(where, fmt.Sprintf("name = %s", nextArg(q.MetricName)))
	where = append(where, fmt.Sprintf("timestamp >= %s", nextArg(q.Start)))
	where = append(where, fmt.Sprintf("timestamp <= %s", nextArg(q.End)))

	for k, v := range q.Filters {
		where = append(where, fmt.Sprintf("tags[%s] = %s", nextArg(k), nextArg(v)))
	}

	groupByClause := "name, toString(tags)"
	if len(q.GroupBy) > 0 {
		parts := []string{"name"}
		for _, g := range q.GroupBy {
			parts = append(parts, fmt.Sprintf("tags[%s]", nextArg(g)))
		}
		groupByClause = strings.Join(parts, ", ")
	}

	sql := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL %d SECOND) AS ts,
			%s(value) AS val,
			name,
			toString(tags) AS tags_str
		FROM metrics		WHERE %s
		GROUP BY ts, %s
		ORDER BY ts ASC
	`, stepSeconds, aggFunc, strings.Join(where, " AND "), groupByClause)

	return sql, args
}

func (e *MetricEngine) buildInstantSQL(q *PromQLQuery) (string, []any) {
	var args []any
	argIdx := 0
	nextArg := func(v any) string {
		argIdx++
		args = append(args, v)
		return fmt.Sprintf("$%d", argIdx)
	}

	var where []string
	where = append(where, fmt.Sprintf("name = %s", nextArg(q.MetricName)))
	where = append(where, fmt.Sprintf("timestamp >= %s", nextArg(q.Start)))
	where = append(where, fmt.Sprintf("timestamp <= %s", nextArg(q.End)))

	for k, v := range q.Filters {
		where = append(where, fmt.Sprintf("tags[%s] = %s", nextArg(k), nextArg(v)))
	}

	sql := fmt.Sprintf(`
		SELECT
			argMax(value, timestamp) AS val,
			toString(tags) AS tags_str
		FROM metrics		WHERE %s
		GROUP BY tags
		ORDER BY val DESC
	`, strings.Join(where, " AND "))

	return sql, args
}

// ParsePromQL parses a simplified PromQL expression.
// Supports: metric_name{label="value", label2="value2"}
// Supports: func(metric{filters}) by (label1, label2)
func ParsePromQL(expr string) (*PromQLQuery, error) {
	expr = strings.TrimSpace(expr)
	q := &PromQLQuery{
		Filters: make(map[string]string),
	}

	// Check for group by: ... by (label1, label2) — must be parsed before function
	byRe := regexp.MustCompile(`\)\s+by\s*\(([^)]+)\)\s*$`)
	if m := byRe.FindStringSubmatch(expr); m != nil {
		for _, l := range strings.Split(m[1], ",") {
			q.GroupBy = append(q.GroupBy, strings.TrimSpace(l))
		}
		// Remove " by (...)" but keep the closing paren of the function
		loc := byRe.FindStringIndex(expr)
		expr = expr[:loc[0]+1] // keep the ")"
	}

	// Check for function wrapping: func(metric{filters})
	funcRe := regexp.MustCompile(`^(\w+)\((.+)\)$`)
	if m := funcRe.FindStringSubmatch(expr); m != nil {
		q.Function = m[1]
		expr = m[2]
	}

	// Parse metric_name{filters}
	braceIdx := strings.Index(expr, "{")
	if braceIdx < 0 {
		q.MetricName = strings.TrimSpace(expr)
		return q, nil
	}

	q.MetricName = strings.TrimSpace(expr[:braceIdx])
	filterStr := expr[braceIdx+1 : strings.LastIndex(expr, "}")]

	// Parse key="value" pairs.
	filterRe := regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
	for _, m := range filterRe.FindAllStringSubmatch(filterStr, -1) {
		q.Filters[m[1]] = m[2]
	}

	return q, nil
}

func parseTags(s string) map[string]string {
	// Simple parser for ClickHouse Map toString output: {'key1':'val1','key2':'val2'}
	result := make(map[string]string)
	s = strings.Trim(s, "{}")
	if s == "" {
		return result
	}
	pairs := strings.Split(s, ",")
	for _, p := range pairs {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) == 2 {
			key := strings.Trim(kv[0], "' ")
			val := strings.Trim(kv[1], "' ")
			result[key] = val
		}
	}
	return result
}
