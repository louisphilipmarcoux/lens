# ADR-001: Monorepo Layout

## Status

Accepted

## Context

Lens consists of multiple components in different languages: Go (agent, ingestion backend, query layer), TypeScript + React (dashboard), and Python (anomaly detection). We need a repository structure that supports all three.

The portfolio PDF suggests a generic `src/` root directory. However, Go projects conventionally use `cmd/` and `internal/` at the repository root. Placing Go code inside `src/` would require either a nested `go.mod` or non-idiomatic import paths.

## Decision

Use Go-idiomatic layout at the repo root (`cmd/`, `internal/`) with separate top-level directories for non-Go components:

- `cmd/` — Go service entry points
- `internal/` — Go internal packages (enforced by Go compiler)
- `web/` — TypeScript + React frontend
- `anomaly/` — Python anomaly detection service
- `deploy/` — Docker and compose files
- `docs/` — Documentation
- `tests/` — Cross-cutting integration and chaos tests

## Consequences

- Go tooling works without configuration (standard layout)
- Each non-Go component has its own package manager config at its root (`web/package.json`, `anomaly/pyproject.toml`)
- A single `go.mod` at the repo root covers all Go code
- The `internal/` directory prevents external imports, which is correct since Lens is an application, not a library
