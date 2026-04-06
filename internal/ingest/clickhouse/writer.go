package clickhouse

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

// BatchWriter buffers incoming data and writes to ClickHouse in batches.
type BatchWriter struct {
	client   *Client
	logger   *zap.Logger
	interval time.Duration
	maxSize  int

	mu      sync.Mutex
	metrics []model.Metric
	logs    []model.LogEntry
	spans   []model.Span
	stopCh  chan struct{}

	// Counters for monitoring.
	MetricsWritten int64
	LogsWritten    int64
	SpansWritten   int64
	WriteErrors    int64
}

// NewBatchWriter creates a writer that flushes to ClickHouse at regular intervals.
func NewBatchWriter(client *Client, interval time.Duration, maxSize int, logger *zap.Logger) *BatchWriter {
	return &BatchWriter{
		client:   client,
		logger:   logger,
		interval: interval,
		maxSize:  maxSize,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic flush loop.
func (w *BatchWriter) Start() {
	go w.flushLoop()
}

// AddMetrics buffers metrics for writing.
func (w *BatchWriter) AddMetrics(metrics []model.Metric) {
	w.mu.Lock()
	w.metrics = append(w.metrics, metrics...)
	shouldFlush := len(w.metrics) >= w.maxSize
	w.mu.Unlock()
	if shouldFlush {
		w.Flush()
	}
}

// AddLogs buffers logs for writing.
func (w *BatchWriter) AddLogs(logs []model.LogEntry) {
	w.mu.Lock()
	w.logs = append(w.logs, logs...)
	shouldFlush := len(w.logs) >= w.maxSize
	w.mu.Unlock()
	if shouldFlush {
		w.Flush()
	}
}

// AddSpans buffers spans for writing.
func (w *BatchWriter) AddSpans(spans []model.Span) {
	w.mu.Lock()
	w.spans = append(w.spans, spans...)
	shouldFlush := len(w.spans) >= w.maxSize
	w.mu.Unlock()
	if shouldFlush {
		w.Flush()
	}
}

// Flush writes all buffered data to ClickHouse.
func (w *BatchWriter) Flush() {
	w.mu.Lock()
	metrics := w.metrics
	logs := w.logs
	spans := w.spans
	w.metrics = nil
	w.logs = nil
	w.spans = nil
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(metrics) > 0 {
		if err := w.client.InsertMetrics(ctx, metrics); err != nil {
			w.logger.Error("failed to write metrics", zap.Int("count", len(metrics)), zap.Error(err))
			w.mu.Lock()
			w.WriteErrors++
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			w.MetricsWritten += int64(len(metrics))
			w.mu.Unlock()
			w.logger.Debug("wrote metrics", zap.Int("count", len(metrics)))
		}
	}

	if len(logs) > 0 {
		if err := w.client.InsertLogs(ctx, logs); err != nil {
			w.logger.Error("failed to write logs", zap.Int("count", len(logs)), zap.Error(err))
			w.mu.Lock()
			w.WriteErrors++
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			w.LogsWritten += int64(len(logs))
			w.mu.Unlock()
			w.logger.Debug("wrote logs", zap.Int("count", len(logs)))
		}
	}

	if len(spans) > 0 {
		if err := w.client.InsertTraces(ctx, spans); err != nil {
			w.logger.Error("failed to write spans", zap.Int("count", len(spans)), zap.Error(err))
			w.mu.Lock()
			w.WriteErrors++
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			w.SpansWritten += int64(len(spans))
			w.mu.Unlock()
			w.logger.Debug("wrote spans", zap.Int("count", len(spans)))
		}
	}
}

// Stop flushes remaining data and stops the writer.
func (w *BatchWriter) Stop() {
	close(w.stopCh)
	w.Flush()
}

func (w *BatchWriter) flushLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.Flush()
		}
	}
}
