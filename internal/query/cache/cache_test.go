package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheSetGet(t *testing.T) {
	c := New(1*time.Minute, 100)

	c.Set("key1", "value1")
	v, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v)

	_, ok = c.Get("missing")
	assert.False(t, ok)
}

func TestCacheExpiry(t *testing.T) {
	c := New(50*time.Millisecond, 100)

	c.Set("key1", "value1")
	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestCacheMaxSize(t *testing.T) {
	c := New(1*time.Minute, 3)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	assert.Equal(t, 3, c.Size())

	c.Set("d", 4) // should evict oldest
	assert.Equal(t, 3, c.Size())
}

func TestCacheKey(t *testing.T) {
	k1 := Key("query1", "param1")
	k2 := Key("query1", "param1")
	k3 := Key("query2", "param1")

	assert.Equal(t, k1, k2)
	assert.NotEqual(t, k1, k3)
	assert.Len(t, k1, 16)
}

func TestCacheHitMiss(t *testing.T) {
	c := New(1*time.Minute, 100)
	c.Set("key1", "value1")

	c.Get("key1")  // hit
	c.Get("key1")  // hit
	c.Get("miss")  // miss

	assert.Equal(t, int64(2), c.Hits)
	assert.Equal(t, int64(1), c.Misses)
}
