// Package clickhouse provides a ClickHouse client for writing observability data.
package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // ClickHouse SQL driver
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
	"github.com/louispm/lens/internal/ingest/schema"
)

// Client wraps a ClickHouse database connection for writing metrics, logs, and traces.
type Client struct {
	db     *sql.DB
	logger *zap.Logger
}

// New creates a ClickHouse client. The dsn should be a clickhouse:// connection string.
func New(dsn string, logger *zap.Logger) (*Client, error) {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &Client{db: db, logger: logger}, nil
}

// Migrate creates tables if they do not exist.
func (c *Client) Migrate(ctx context.Context) error {
	for i, ddl := range schema.Migrations {
		if _, err := c.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	c.logger.Info("clickhouse migrations complete", zap.Int("count", len(schema.Migrations)))
	return nil
}

// InsertMetrics writes a batch of metrics.
func (c *Client) InsertMetrics(ctx context.Context, metrics []model.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO metrics (name, tags, value, type, timestamp) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range metrics {
		metricType := "gauge"
		switch m.Type {
		case model.MetricCounter:
			metricType = "counter"
		case model.MetricHistogram:
			metricType = "histogram"
		}

		if _, err := stmt.ExecContext(ctx, m.Name, mapToClickhouse(m.Tags), m.Value, metricType, m.Timestamp); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert metric %s: %w", m.Name, err)
		}
	}

	return tx.Commit()
}

// InsertLogs writes a batch of log entries.
func (c *Client) InsertLogs(ctx context.Context, logs []model.LogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO logs (timestamp, service, host, level, message, fields, source) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, l := range logs {
		fields := stringifyFields(l.Fields)
		if _, err := stmt.ExecContext(ctx, l.Timestamp, l.Service, l.Host, l.Level, l.Message, fields, l.Source); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert log: %w", err)
		}
	}

	return tx.Commit()
}

// InsertTraces writes a batch of spans.
func (c *Client) InsertTraces(ctx context.Context, spans []model.Span) error {
	if len(spans) == 0 {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO traces (trace_id, span_id, parent_id, service, operation, start_time, duration_ns, status, tags, events) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, s := range spans {
		status := "unset"
		switch s.Status {
		case model.SpanStatusOK:
			status = "ok"
		case model.SpanStatusError:
			status = "error"
		}

		eventsJSON, _ := json.Marshal(s.Events)

		if _, err := stmt.ExecContext(ctx,
			s.TraceID, s.SpanID, s.ParentID,
			s.Service, s.Operation,
			s.StartTime, uint64(s.Duration.Nanoseconds()),
			status, mapToClickhouse(s.Tags),
			string(eventsJSON),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert span %s: %w", s.SpanID, err)
		}
	}

	return tx.Commit()
}

// Close closes the database connection.
func (c *Client) Close() error {
	return c.db.Close()
}

// mapToClickhouse converts a map to a ClickHouse Map(String, String) compatible format.
func mapToClickhouse(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// stringifyFields converts map[string]any to map[string]string for ClickHouse.
func stringifyFields(m map[string]any) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
