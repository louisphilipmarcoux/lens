package batcher

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

func TestBatcherFlushOnSize(t *testing.T) {
	var mu sync.Mutex
	var batches []model.Batch

	output := func(data []byte) error {
		var b model.Batch
		_ = json.Unmarshal(data, &b)
		mu.Lock()
		batches = append(batches, b)
		mu.Unlock()
		return nil
	}

	logger := zap.NewNop()
	b := New(5, 10*time.Minute, output, logger) // very long maxWait so it won't fire
	defer b.Stop()

	metrics := make([]model.Metric, 5)
	for i := range metrics {
		metrics[i] = model.Metric{Name: "test", Value: float64(i)}
	}
	b.AddMetrics(metrics)

	mu.Lock()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0].Metrics, 5)
	mu.Unlock()
}

func TestBatcherFlushOnStop(t *testing.T) {
	var received []byte

	output := func(data []byte) error {
		received = data
		return nil
	}

	logger := zap.NewNop()
	b := New(100, 10*time.Minute, output, logger)

	b.AddMetrics([]model.Metric{{Name: "test", Value: 42}})
	b.Stop()

	require.NotNil(t, received)
	var batch model.Batch
	require.NoError(t, json.Unmarshal(received, &batch))
	assert.Len(t, batch.Metrics, 1)
}

func TestBatcherMixedTypes(t *testing.T) {
	var received model.Batch

	output := func(data []byte) error {
		_ = json.Unmarshal(data, &received)
		return nil
	}

	logger := zap.NewNop()
	b := New(100, 10*time.Minute, output, logger)

	b.AddMetrics([]model.Metric{{Name: "cpu"}})
	b.AddLogs([]model.LogEntry{{Message: "hello"}})
	b.AddSpans([]model.Span{{TraceID: "abc123"}})
	b.Stop()

	assert.Len(t, received.Metrics, 1)
	assert.Len(t, received.Logs, 1)
	assert.Len(t, received.Spans, 1)
}
