# ADR-002: Disk-Backed WAL Buffer

## Status

Accepted

## Context

The collector agent must not lose data when the ingestion backend is temporarily unavailable. Options considered:

1. **In-memory ring buffer** — Simple, but loses all data on agent crash or restart.
2. **Channel-based buffering** — Bounded channels block producers when full; unbounded channels consume unbounded memory.
3. **Disk-backed WAL (write-ahead log)** — Survives agent restarts and backend outages. Trades disk I/O for durability.

## Decision

Use a disk-backed WAL with append-only segment files. Each entry is framed as:

```
[4-byte length (big-endian uint32)][payload][4-byte CRC32 checksum]
```

Segments are rotated when they reach a configurable maximum size (default 64MB). Fully shipped segments are deleted. When total disk usage exceeds the configured limit, the oldest unshipped segment is evicted and a data-loss counter is incremented.

## Consequences

- **Durability**: Data survives agent crashes and backend outages (up to configured disk limit).
- **Corruption detection**: CRC32 checksums detect corrupted entries. Corrupted entries are skipped rather than causing the agent to crash.
- **Backpressure**: The disk buffer provides natural backpressure — when the backend is slow, disk fills up, and eventually the oldest data is evicted. This is explicit and observable via the data-loss counter.
- **Performance**: Append-only writes are efficient. Sequential reads for shipping are also efficient. The overhead is acceptable for an agent that collects data at ~10 second intervals.
- **Complexity**: More complex than in-memory alternatives, but the durability guarantee is essential for a production observability agent.
