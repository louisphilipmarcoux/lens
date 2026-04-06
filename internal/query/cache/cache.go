// Package cache provides an in-memory TTL cache for query results.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type entry struct {
	value     any
	expiresAt time.Time
}

// Cache is a thread-safe in-memory cache with TTL expiration.
type Cache struct {
	mu      sync.RWMutex
	items   map[string]entry
	ttl     time.Duration
	maxSize int

	Hits   int64
	Misses int64
}

// New creates a cache with the given TTL and max entries.
func New(ttl time.Duration, maxSize int) *Cache {
	c := &Cache{
		items:   make(map[string]entry),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.evictLoop()
	return c
}

// Get retrieves a cached value. Returns nil and false if not found or expired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		c.mu.Lock()
		c.Misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.Hits++
	c.mu.Unlock()
	return e.value, true
}

// Set stores a value with the default TTL.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Key generates a cache key from a query string and parameters.
func Key(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, e := range c.items {
		if first || e.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.expiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}
