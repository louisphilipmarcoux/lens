package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiterAllows(t *testing.T) {
	rl := NewRateLimiter(5)

	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(), "request %d should be allowed", i)
	}

	assert.False(t, rl.Allow(), "6th request should be rejected")
}

func TestRateLimiterRefills(t *testing.T) {
	rl := NewRateLimiter(3)

	// Exhaust tokens.
	for i := 0; i < 3; i++ {
		rl.Allow()
	}
	assert.False(t, rl.Allow())

	// Simulate time passing by resetting lastFill.
	rl.mu.Lock()
	rl.lastFill = rl.lastFill.Add(-2 * 1e9)
	rl.mu.Unlock()

	assert.True(t, rl.Allow(), "should allow after refill")
}
