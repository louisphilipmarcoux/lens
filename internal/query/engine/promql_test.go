package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePromQL_SimpleMetric(t *testing.T) {
	q, err := ParsePromQL("cpu.user_percent")
	require.NoError(t, err)
	assert.Equal(t, "cpu.user_percent", q.MetricName)
	assert.Empty(t, q.Filters)
	assert.Empty(t, q.Function)
}

func TestParsePromQL_WithFilters(t *testing.T) {
	q, err := ParsePromQL(`cpu.user_percent{host="web-01", cpu="cpu0"}`)
	require.NoError(t, err)
	assert.Equal(t, "cpu.user_percent", q.MetricName)
	assert.Equal(t, "web-01", q.Filters["host"])
	assert.Equal(t, "cpu0", q.Filters["cpu"])
}

func TestParsePromQL_WithFunction(t *testing.T) {
	q, err := ParsePromQL(`avg(cpu.user_percent{host="web-01"})`)
	require.NoError(t, err)
	assert.Equal(t, "avg", q.Function)
	assert.Equal(t, "cpu.user_percent", q.MetricName)
	assert.Equal(t, "web-01", q.Filters["host"])
}

func TestParsePromQL_WithGroupBy(t *testing.T) {
	q, err := ParsePromQL(`sum(http_requests_total{service="api"}) by (method, status)`)
	require.NoError(t, err)
	assert.Equal(t, "sum", q.Function)
	assert.Equal(t, "http_requests_total", q.MetricName)
	assert.Equal(t, "api", q.Filters["service"])
	assert.Equal(t, []string{"method", "status"}, q.GroupBy)
}

func TestParseTags(t *testing.T) {
	tags := parseTags("{'host':'web-01','cpu':'cpu0'}")
	assert.Equal(t, "web-01", tags["host"])
	assert.Equal(t, "cpu0", tags["cpu"])
}

func TestParseTags_Empty(t *testing.T) {
	tags := parseTags("{}")
	assert.Empty(t, tags)
}
