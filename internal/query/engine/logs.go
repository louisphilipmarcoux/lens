package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LogQuery represents a structured log search.
type LogQuery struct {
	Service   string
	Level     string
	Search    string // substring match on message
	Fields    map[string]string
	Start     time.Time
	End       time.Time
	Limit     int
	Offset    int
	OrderDesc bool
}

// LogRecord represents a log entry from the query result.
type LogRecord struct {
	Timestamp time.Time         `json:"timestamp"`
	Service   string            `json:"service"`
	Host      string            `json:"host"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields"`
	Source    string            `json:"source"`
}

// LogAggregation represents the result of a log aggregation query.
type LogAggregation struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// LogEngine executes log queries against ClickHouse.
type LogEngine struct {
	db *sql.DB
}

// NewLogEngine creates a log query engine.
func NewLogEngine(db *sql.DB) *LogEngine {
	return &LogEngine{db: db}
}

// Search executes a log search query.
func (e *LogEngine) Search(ctx context.Context, q *LogQuery) ([]LogRecord, int64, error) {
	whereClause, args := e.buildWhere(q)

	// Count total.
	var total int64
	countSQL := fmt.Sprintf("SELECT count() FROM logs FINAL WHERE %s", whereClause)
	if err := e.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count logs: %w", err)
	}

	// Fetch results.
	order := "ASC"
	if q.OrderDesc {
		order = "DESC"
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}

	querySQL := fmt.Sprintf(`
		SELECT timestamp, service, host, level, message, toString(fields), source
		FROM logs FINAL
		WHERE %s
		ORDER BY timestamp %s
		LIMIT %d OFFSET %d
	`, whereClause, order, limit, q.Offset)

	rows, err := e.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query logs: %w", err)
	}
	defer rows.Close()

	var records []LogRecord
	for rows.Next() {
		var r LogRecord
		var fieldsStr string
		if err := rows.Scan(&r.Timestamp, &r.Service, &r.Host, &r.Level, &r.Message, &fieldsStr, &r.Source); err != nil {
			return nil, 0, fmt.Errorf("scan log: %w", err)
		}
		r.Fields = parseTags(fieldsStr)
		records = append(records, r)
	}

	return records, total, rows.Err()
}

// Aggregate returns log counts grouped by a field.
func (e *LogEngine) Aggregate(ctx context.Context, q *LogQuery, groupBy string) ([]LogAggregation, error) {
	whereClause, args := e.buildWhere(q)

	var groupExpr string
	switch groupBy {
	case "level", "service", "host":
		groupExpr = groupBy
	default:
		groupExpr = fmt.Sprintf("fields['%s']", groupBy)
	}

	querySQL := fmt.Sprintf(`
		SELECT %s AS key, count() AS cnt
		FROM logs FINAL
		WHERE %s
		GROUP BY key
		ORDER BY cnt DESC
		LIMIT 100
	`, groupExpr, whereClause)

	rows, err := e.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregate logs: %w", err)
	}
	defer rows.Close()

	var results []LogAggregation
	for rows.Next() {
		var a LogAggregation
		if err := rows.Scan(&a.Key, &a.Count); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (e *LogEngine) buildWhere(q *LogQuery) (string, []any) {
	var where []string
	var args []any
	idx := 0
	nextArg := func(v any) string {
		idx++
		args = append(args, v)
		return fmt.Sprintf("$%d", idx)
	}

	if !q.Start.IsZero() {
		where = append(where, fmt.Sprintf("timestamp >= %s", nextArg(q.Start)))
	}
	if !q.End.IsZero() {
		where = append(where, fmt.Sprintf("timestamp <= %s", nextArg(q.End)))
	}
	if q.Service != "" {
		where = append(where, fmt.Sprintf("service = %s", nextArg(q.Service)))
	}
	if q.Level != "" {
		where = append(where, fmt.Sprintf("level = %s", nextArg(q.Level)))
	}
	if q.Search != "" {
		where = append(where, fmt.Sprintf("message ILIKE %s", nextArg("%"+q.Search+"%")))
	}
	for k, v := range q.Fields {
		where = append(where, fmt.Sprintf("fields[%s] = %s", nextArg(k), nextArg(v)))
	}

	if len(where) == 0 {
		return "1 = 1", nil
	}
	return strings.Join(where, " AND "), args
}
