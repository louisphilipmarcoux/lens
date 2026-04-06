# Lens — Observability Platform

[![CI](https://github.com/louispm/lens/actions/workflows/ci.yml/badge.svg)](https://github.com/louispm/lens/actions/workflows/ci.yml)

A production-grade observability platform for metrics, logs, and traces — built from scratch in Go, TypeScript, and Python. Equivalent to a focused Datadog/Grafana.

**Project 03** of the [Portfolio Engineering Program](https://github.com/louispm). Monitors systems built in Projects [01 (Container Runtime)](https://github.com/louispm/vessel) and [02 (KV Store)](https://github.com/louispm/kv).

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
│ /proc, eBPF │     └──────────────┘     └────────────┘     └─────┬─────┘
│ logs, OTel  │                                                    │
└─────────────┘                                              ┌─────▼─────┐
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

## Production Scope

### In Scope (Production Quality)

- Collector agent: host metrics via /proc, log tailing, OTel span ingestion
- eBPF-based kernel metrics (TCP retransmits, syscall latency, disk I/O)
- Disk-backed agent buffer — zero data loss when backend is down
- Horizontal ingestion backend with ClickHouse storage
- PromQL-compatible metric query language
- Log filter and aggregate DSL
- Distributed trace storage with flame graph rendering
- Real-time metric streaming via WebSocket
- Dashboard-as-code (JSON config stored in KV store)
- Alerting engine with deduplication, routing, webhook + email notifications
- Metric anomaly detection via seasonal decomposition
- Data retention policies per signal type

### Explicitly Out of Scope

- Multi-tenant billing and access control
- Global multi-region deployment
- Mobile app
- SLA / error budget tracking
- Synthetic monitoring
- Custom plugin system

See [docs/limitations.md](docs/limitations.md) for detailed rationale on each exclusion.

## Tech Stack

| Component | Language | Key Libraries |
|-----------|----------|---------------|
| Agent | Go | cilium/ebpf, fsnotify, OTel proto, Prometheus client |
| Ingestion Backend | Go | ClickHouse driver, gRPC, protobuf |
| Query Layer | Go | PromQL parser, gRPC, WebSocket |
| Dashboard | TypeScript + React | Tailwind CSS, WebSocket, flame graph renderer |
| Anomaly Detection | Python | statsmodels, NumPy |
| Storage | ClickHouse | Time-series + columnar storage |

## Quick Start

### Prerequisites

- Go 1.23+
- Docker and Docker Compose
- Node.js 20+ (for frontend)
- Python 3.11+ (for anomaly detection)

### Build

```bash
make build
```

### Run Tests

```bash
make test        # unit tests
make test-race   # unit tests with race detector
```

### Local Development

```bash
make dev         # starts ClickHouse + agent via Docker Compose
```

## Project Structure

```
cmd/                    Go service entry points
  agent/                Collector agent
  ingest/               Ingestion backend
  query/                Query API server
internal/               Go internal packages
  agent/                Agent subsystems (collectors, buffer, shipper)
  common/               Shared models and protobuf definitions
web/                    React + TypeScript dashboard
anomaly/                Python anomaly detection service
deploy/                 Docker and compose files
docs/                   Architecture decisions, internals, limitations
tests/                  Integration and chaos tests
```

## Documentation

- [Architecture Decisions](docs/decisions/) — ADRs for major technical choices
- [Internals](docs/internals/) — Deep-dive into core algorithms
- [Known Limitations](docs/limitations.md) — Honest scope boundaries and future work

## License

MIT — see [LICENSE](LICENSE).
