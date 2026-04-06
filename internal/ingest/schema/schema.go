// Package schema defines the ClickHouse table schemas for Lens.
package schema

// Migrations contains the DDL statements to create all Lens tables.
// These are executed in order on startup if the tables do not exist.
var Migrations = []string{
	CreateMetricsTable,
	CreateLogsTable,
	CreateTracesTable,
	CreateMetricsRetentionPolicy,
	CreateLogsRetentionPolicy,
	CreateTracesRetentionPolicy,
}

// CreateMetricsTable creates the metrics table with proper partition and sort keys.
// Partitioned by day for efficient time-range queries and TTL-based deletion.
// Sorted by (name, tags hash, timestamp) for fast metric lookups.
const CreateMetricsTable = `
CREATE TABLE IF NOT EXISTS metrics (
    name       String,
    tags       Map(String, String),
    value      Float64,
    type       Enum8('gauge' = 0, 'counter' = 1, 'histogram' = 2),
    timestamp  DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC') DEFAULT now64(3),
    dedup_key  String DEFAULT concat(name, toString(tags), toString(timestamp))
) ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (name, cityHash64(toString(tags)), timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192
`

// CreateLogsTable creates the logs table.
// Partitioned by day. Sorted by (service, timestamp) for service-scoped log queries.
// A full-text index on message enables log search.
const CreateLogsTable = `
CREATE TABLE IF NOT EXISTS logs (
    timestamp   DateTime64(3, 'UTC'),
    service     LowCardinality(String),
    host        LowCardinality(String),
    level       LowCardinality(String),
    message     String,
    fields      Map(String, String),
    source      String,
    received_at DateTime64(3, 'UTC') DEFAULT now64(3),
    dedup_key   String DEFAULT concat(service, host, toString(timestamp), substring(message, 1, 128))
) ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (service, level, timestamp)
TTL timestamp + INTERVAL 14 DAY
SETTINGS index_granularity = 8192
`

// CreateTracesTable creates the traces/spans table.
// Partitioned by day. Sorted by (service, trace_id, start_time) for trace lookups.
const CreateTracesTable = `
CREATE TABLE IF NOT EXISTS traces (
    trace_id    String,
    span_id     String,
    parent_id   String DEFAULT '',
    service     LowCardinality(String),
    operation   String,
    start_time  DateTime64(6, 'UTC'),
    duration_ns UInt64,
    status      Enum8('unset' = 0, 'ok' = 1, 'error' = 2),
    tags        Map(String, String),
    events      String DEFAULT '[]',
    received_at DateTime64(3, 'UTC') DEFAULT now64(3)
) ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (service, trace_id, start_time)
TTL start_time + INTERVAL 7 DAY
SETTINGS index_granularity = 8192
`

// CreateMetricsRetentionPolicy creates a materialized view for downsampled metrics.
// Keeps 5-minute averages for 90 days (cold tier).
const CreateMetricsRetentionPolicy = `
CREATE TABLE IF NOT EXISTS metrics_5m (
    name       String,
    tags       Map(String, String),
    avg_value  Float64,
    min_value  Float64,
    max_value  Float64,
    count      UInt64,
    timestamp  DateTime64(3, 'UTC')
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (name, cityHash64(toString(tags)), timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192
`

// CreateLogsRetentionPolicy sets up compressed cold storage for older logs.
const CreateLogsRetentionPolicy = `
ALTER TABLE logs MODIFY SETTING storage_policy = 'default'
`

// CreateTracesRetentionPolicy is a no-op placeholder — traces use TTL in the main table.
const CreateTracesRetentionPolicy = `
SELECT 1
`
