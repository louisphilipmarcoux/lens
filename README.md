# Lens — Observability Platform

[![CI](https://github.com/louisphilipmarcoux/lens/actions/workflows/ci.yml/badge.svg)](https://github.com/louisphilipmarcoux/lens/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python&logoColor=white)](https://python.org)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-24.8-FFCC01?logo=clickhouse&logoColor=black)](https://clickhouse.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Lines of Code](https://img.shields.io/badge/Lines-~10k-blue)

A production-grade observability platform for metrics, logs, and traces — built from scratch in Go, TypeScript, and Python. Equivalent to a focused Datadog/Grafana.

**Project 03** of the [Portfolio Engineering Program](https://github.com/louisphilipmarcoux). Monitors systems built in Projects [01 (Container Runtime)](https://github.com/louisphilipmarcoux/vessel) and [02 (KV Store)](https://github.com/louisphilipmarcoux/kv).

---

## What It Does

Lens collects, stores, queries, and alerts on three signal types:

- **Metrics** — Host-level (/proc) and kernel-level (eBPF) metrics, collected by a Go agent
- **Logs** — Structured log ingestion with schema-on-write parsing
- **Traces** — OpenTelemetry-compatible distributed trace collection

All three signals flow through a unified pipeline: **Agent → Ingestion Backend → ClickHouse → Query Layer → Dashboard**.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐     ┌───────────┐
│  Lens Agent  │────▶│   Ingestion  │────▶│ ClickHouse │◀────│   Query   │
│  (Go)        │     │   Backend    │     │  Storage   │     │   Layer   │
│              │     │  (Go)        │     │            │     │  (Go)     │
│ /proc, eBPF  │     └──────────────┘     └────────────┘     └─────┬─────┘
│ logs, OTel   │                                                   │
└──────────────┘                                             ┌─────▼─────┐
                                                             │ Dashboard │
                    ┌──────────────┐                         │ (React)   │
                    │   Alerting   │◀── PromQL eval          └───────────┘
                    │   Engine     │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   Anomaly    │
                    │  Detection   │
                    │  (Python)    │
                    └──────────────┘
```

## Key Features

| Feature | Description |
| ------- | ----------- |
| /proc Collectors | CPU, memory, disk, network, load average with cross-platform testdata |
| eBPF Metrics | TCP retransmits, syscall latency histograms, block I/O latency via kernel tracepoints |
| Log Tailing | File tailer with JSON/raw parsing, rotation detection, offset persistence |
| OTel Traces | gRPC OTLP trace receiver compatible with OpenTelemetry SDKs |
| Zero Data Loss | Disk-backed WAL buffer with CRC32 checksums survives backend outages |
| ClickHouse Storage | ReplacingMergeTree for dedup, daily partitions, TTL retention (30d/14d/7d) |
| PromQL Queries | Simplified PromQL parser with instant/range queries, aggregation, group by |
| Log Search | Filter by service/level/message, pagination, field aggregation |
| Trace Viewer | Span tree assembly, flame graph visualization, correlated drill-down |
| Real-Time Streaming | Server-sent events for live metric charts |
| Alerting | Rule evaluation with for-duration, dedup fingerprinting, webhook + email |
| Anomaly Detection | Seasonal decomposition with adaptive thresholds, multi-metric correlation |
| Dashboard | React + Tailwind with real-time charts, log explorer, trace flame graph |

## Production Scope

### In Scope (Production Quality)

- Collector agent: host metrics via /proc, log tailing, OTel span ingestion
- eBPF-based kernel metrics (TCP retransmits, syscall latency, disk I/O)
- Disk-backed agent buffer with CRC32 integrity checks
- Horizontal ingestion backend with ClickHouse storage
- PromQL-compatible metric query language
- Log filter and aggregate DSL
- Distributed trace storage with flame graph rendering
- Real-time metric streaming via SSE
- Dashboard-as-code (JSON config stored in KV store)
- Alerting engine with deduplication, routing, webhook + email notifications
- Metric anomaly detection via seasonal decomposition
- Data retention policies per signal type (tiered hot/cold)

### Explicitly Out of Scope

- Multi-tenant billing and access control
- Global multi-region deployment
- Mobile app
- SLA / error budget tracking
- Synthetic monitoring
- Custom plugin system

See [docs/limitations.md](docs/limitations.md) for detailed rationale and estimated effort for each exclusion.

## Tech Stack

| Component | Language | Key Libraries |
|-----------|----------|---------------|
| Agent | Go | cilium/ebpf, fsnotify, OTel proto, Prometheus client |
| Ingestion Backend | Go | ClickHouse driver, gRPC, protobuf |
| Query Layer | Go | PromQL parser, SSE streaming, in-memory cache |
| Dashboard | TypeScript + React | Recharts, Tailwind CSS, React Router |
| Anomaly Detection | Python | NumPy, statsmodels |
| Storage | ClickHouse | Time-series + columnar storage |

## Quick Start

### Prerequisites

- Go 1.24+
- Docker and Docker Compose
- Node.js 20+ (for frontend)
- Python 3.11+ (for anomaly detection)

### Build

```bash
make build         # build all Go binaries
```

### Run Tests

```bash
make test          # unit tests
make test-race     # unit tests with race detector
make lint          # golangci-lint
```

### Local Development

```bash
make dev           # starts ClickHouse + ingest + agent via Docker Compose
cd web && npm dev  # starts frontend dev server on :3000
```

## Project Structure

```
cmd/
  agent/                Collector agent entry point
  ingest/               Ingestion backend entry point
  query/                Query API server entry point
internal/
  agent/                Agent: collectors, buffer, batcher, shipper, selfmon
  alert/                Alerting engine with rule evaluation and notifiers
  common/model/         Shared data types (Metric, LogEntry, Span, Batch)
  ingest/               Ingestion: ClickHouse client, schema, batch writer, API
  query/                Query: PromQL engine, log/trace engines, cache, middleware
web/                    React + TypeScript + Tailwind dashboard
anomaly/                Python anomaly detection service
deploy/                 Dockerfiles and Docker Compose
docs/
  decisions/            Architecture Decision Records (ADRs)
  internals/            Deep-dive documentation
  limitations.md        Honest scope boundaries and future work
tests/
  integration/          End-to-end pipeline tests
  chaos/                Chaos test scenarios
```

## Documentation

- [Architecture Decisions](docs/decisions/) — ADRs for major technical choices
  - [001 — Monorepo Layout](docs/decisions/001-monorepo-layout.md)
  - [002 — Disk-Backed Buffer](docs/decisions/002-disk-backed-buffer.md)
  - [003 — /proc Parsing Strategy](docs/decisions/003-proc-parsing-strategy.md)
  - [004 — eBPF Kernel Metrics](docs/decisions/004-ebpf-kernel-metrics.md)
  - [005 — ClickHouse Schema](docs/decisions/005-clickhouse-schema.md)
- [Internals](docs/internals/) — Deep-dive into core algorithms
- [Known Limitations](docs/limitations.md) — Honest scope boundaries and future work

## Integration With Portfolio Projects

Lens is designed to monitor the other two portfolio projects end-to-end:

- **Container Runtime** (Project 01) — cgroup metrics per container, lifecycle events, structured logs
- **Distributed KV Store** (Project 02) — Raft election metrics, replication lag, OTel traces, Watch API config sync

See [docs/internals/instrumentation.md](docs/internals/instrumentation.md) for the full integration guide.

## License

MIT — see [LICENSE](LICENSE).
