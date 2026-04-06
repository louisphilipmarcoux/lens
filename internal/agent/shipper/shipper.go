package shipper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/agent/buffer"
)

// Shipper reads batches from the disk buffer and sends them to the backend.
type Shipper struct {
	buf         *buffer.DiskBuffer
	backendURL  string
	timeout     time.Duration
	concurrency int
	logger      *zap.Logger
	client      *http.Client

	shipped atomic.Int64
	failed  atomic.Int64
}

// New creates a Shipper.
func New(buf *buffer.DiskBuffer, backendURL string, timeout time.Duration, logger *zap.Logger) *Shipper {
	return &Shipper{
		buf:         buf,
		backendURL:  backendURL,
		timeout:     timeout,
		concurrency: 4,
		logger:      logger,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Run starts the shipping loop. It blocks until the context is cancelled.
func (s *Shipper) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Final flush attempt.
			s.shipOnce(context.Background())
			return
		default:
		}

		shipped := s.shipOnce(ctx)
		if !shipped {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func (s *Shipper) shipOnce(ctx context.Context) bool {
	entries, segmentPath, err := s.buf.ReadAll()
	if err != nil {
		s.logger.Error("failed to read from buffer", zap.Error(err))
		return false
	}
	if segmentPath == "" || len(entries) == 0 {
		return false
	}

	allOK := true
	for _, entry := range entries {
		if err := s.send(ctx, entry); err != nil {
			s.logger.Warn("failed to ship batch", zap.Error(err))
			s.failed.Add(1)
			allOK = false
			break
		}
		s.shipped.Add(1)
	}

	if allOK {
		if err := s.buf.Ack(segmentPath); err != nil {
			s.logger.Error("failed to ack segment", zap.String("path", segmentPath), zap.Error(err))
		}
	}

	return true
}

func (s *Shipper) send(ctx context.Context, data []byte) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			wait := Backoff(attempt, 1*time.Second, 30*time.Second)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.backendURL+"/api/v1/ingest", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("backend returned status %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr // don't retry client errors
		}
	}
	return fmt.Errorf("exhausted retries: %w", lastErr)
}

// ShippedCount returns the total number of successfully shipped batches.
func (s *Shipper) ShippedCount() int64 { return s.shipped.Load() }

// FailedCount returns the total number of failed ship attempts.
func (s *Shipper) FailedCount() int64 { return s.failed.Load() }

// Backoff computes an exponential backoff duration with jitter.
func Backoff(attempt int, base, max time.Duration) time.Duration {
	d := base * time.Duration(1<<uint(attempt))
	if d > max {
		d = max
	}
	jitter := time.Duration(rand.Int63n(int64(d)/2 + 1))
	return d/2 + jitter
}
