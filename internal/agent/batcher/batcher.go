package batcher

import (
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

// OutputFunc is called when a batch is ready to be written.
type OutputFunc func(data []byte) error

// Batcher accumulates metrics, logs, and spans, flushing them as serialized
// batches when either a size threshold or time limit is reached.
type Batcher struct {
	maxSize int
	maxWait time.Duration
	output  OutputFunc
	logger  *zap.Logger

	mu      sync.Mutex
	batch   model.Batch
	count   int
	timer   *time.Timer
	stopCh  chan struct{}
	stopped bool
}

// New creates a Batcher that flushes when maxSize items accumulate or maxWait elapses.
func New(maxSize int, maxWait time.Duration, output OutputFunc, logger *zap.Logger) *Batcher {
	b := &Batcher{
		maxSize: maxSize,
		maxWait: maxWait,
		output:  output,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
	b.resetTimer()
	return b
}

// AddMetrics adds metrics to the current batch.
func (b *Batcher) AddMetrics(metrics []model.Metric) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch.Metrics = append(b.batch.Metrics, metrics...)
	b.count += len(metrics)
	if b.count >= b.maxSize {
		b.flushLocked()
	}
}

// AddLogs adds log entries to the current batch.
func (b *Batcher) AddLogs(logs []model.LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch.Logs = append(b.batch.Logs, logs...)
	b.count += len(logs)
	if b.count >= b.maxSize {
		b.flushLocked()
	}
}

// AddSpans adds spans to the current batch.
func (b *Batcher) AddSpans(spans []model.Span) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batch.Spans = append(b.batch.Spans, spans...)
	b.count += len(spans)
	if b.count >= b.maxSize {
		b.flushLocked()
	}
}

// Flush forces a flush of the current batch.
func (b *Batcher) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// Stop flushes remaining data and stops the timer.
func (b *Batcher) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	b.stopped = true
	b.flushLocked()
	b.timer.Stop()
	close(b.stopCh)
}

func (b *Batcher) flushLocked() {
	if b.count == 0 {
		return
	}

	data, err := json.Marshal(b.batch)
	if err != nil {
		b.logger.Error("failed to serialize batch", zap.Error(err))
		b.resetBatchLocked()
		return
	}

	if err := b.output(data); err != nil {
		b.logger.Error("failed to write batch", zap.Error(err))
	} else {
		b.logger.Debug("batch flushed", zap.Int("items", b.count), zap.Int("bytes", len(data)))
	}

	b.resetBatchLocked()
	b.resetTimer()
}

func (b *Batcher) resetBatchLocked() {
	b.batch = model.Batch{}
	b.count = 0
}

func (b *Batcher) resetTimer() {
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.maxWait, func() {
		b.Flush()
	})
}
