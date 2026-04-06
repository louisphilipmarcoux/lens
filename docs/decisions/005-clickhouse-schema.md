# ADR-005: ClickHouse Schema Design

## Status

Accepted

## Context

Lens needs a storage engine for metrics, logs, and traces. Requirements:
- High write throughput (target: 1M metrics/min)
- Fast time-range queries
- Columnar compression for cost-effective storage
- TTL-based data retention

Options considered:
1. **TimescaleDB** — PostgreSQL extension. Good SQL support, but slower writes at scale.
2. **InfluxDB** — Time-series native, but limited log/trace support.
3. **ClickHouse** — Columnar, very high write throughput, good compression, SQL interface.

## Decision

Use ClickHouse with three main tables:

### Metrics Table
- **Engine**: `ReplacingMergeTree(received_at)` — deduplicates by (name, tags hash, timestamp) keeping the latest `received_at`
- **Partition**: `toYYYYMMDD(timestamp)` — one partition per day for efficient time-range pruning and TTL deletion
- **Sort key**: `(name, cityHash64(toString(tags)), timestamp)` — fast metric name + tag lookups
- **TTL**: 30 days at full resolution; 90 days at 5-minute downsampled aggregates

### Logs Table
- **Engine**: `ReplacingMergeTree(received_at)` — dedup by (service, host, timestamp, message prefix)
- **Partition**: `toYYYYMMDD(timestamp)` — daily partitions
- **Sort key**: `(service, level, timestamp)` — fast service-scoped queries with level filtering
- **TTL**: 14 days
- **LowCardinality**: Applied to service, host, level for dictionary compression

### Traces Table
- **Engine**: `ReplacingMergeTree(received_at)` — dedup by (service, trace_id, start_time)
- **Partition**: `toYYYYMMDD(start_time)` — daily partitions
- **Sort key**: `(service, trace_id, start_time)` — fast trace-by-ID lookups scoped to service
- **TTL**: 7 days

## Consequences

- **Deduplication**: `ReplacingMergeTree` handles exactly-once semantics. Duplicates from agent retries are collapsed during background merges. `FINAL` keyword forces dedup at query time.
- **Partition pruning**: Daily partitions mean queries with time range filters only scan relevant partitions.
- **Compression**: ClickHouse's columnar format achieves 10-20x compression on metrics data. LowCardinality strings compress further.
- **TTL**: Automatic data expiration per signal type. No manual cleanup needed.
- **Downsampling**: A separate `metrics_5m` table stores 5-minute aggregates for long-term queries. A materialized view or scheduled job populates it.
