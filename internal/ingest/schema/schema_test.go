package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrationsNotEmpty(t *testing.T) {
	assert.NotEmpty(t, Migrations)
	for i, m := range Migrations {
		assert.NotEmpty(t, strings.TrimSpace(m), "migration %d is empty", i)
	}
}

func TestMetricsTableHasPartition(t *testing.T) {
	assert.Contains(t, CreateMetricsTable, "PARTITION BY")
	assert.Contains(t, CreateMetricsTable, "ORDER BY")
	assert.Contains(t, CreateMetricsTable, "TTL")
	assert.Contains(t, CreateMetricsTable, "ReplacingMergeTree")
}

func TestLogsTableHasFullTextFields(t *testing.T) {
	assert.Contains(t, CreateLogsTable, "message")
	assert.Contains(t, CreateLogsTable, "service")
	assert.Contains(t, CreateLogsTable, "level")
	assert.Contains(t, CreateLogsTable, "PARTITION BY")
}

func TestTracesTableHasTraceFields(t *testing.T) {
	assert.Contains(t, CreateTracesTable, "trace_id")
	assert.Contains(t, CreateTracesTable, "span_id")
	assert.Contains(t, CreateTracesTable, "duration_ns")
	assert.Contains(t, CreateTracesTable, "PARTITION BY")
}
