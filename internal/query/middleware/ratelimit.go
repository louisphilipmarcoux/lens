// Package middleware provides HTTP middleware for the query layer.
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	maxRate  int
	lastFill time.Time
}

// NewRateLimiter creates a rate limiter that allows maxRate requests per second.
func NewRateLimiter(maxRate int) *RateLimiter {
	return &RateLimiter{
		tokens:   maxRate,
		maxRate:  maxRate,
		lastFill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastFill)
	if elapsed >= time.Second {
		rl.tokens = rl.maxRate
		rl.lastFill = now
	}

	if rl.tokens <= 0 {
		return false
	}
	rl.tokens--
	return true
}

// RateLimit returns HTTP middleware that enforces the rate limit.
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout returns HTTP middleware that enforces a query timeout.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"query timeout"}`)
	}
}
