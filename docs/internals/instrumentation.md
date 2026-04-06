# Instrumenting the Portfolio Stack

This document describes how to instrument the Container Runtime (Project 01) and
Distributed KV Store (Project 02) into the Lens observability platform.

## Container Runtime Instrumentation

### Metrics Exported

| Metric | Source | Type |
|--------|--------|------|
| `container.cpu_usage_percent` | cgroups v2 cpu.stat | gauge |
| `container.memory_usage_bytes` | cgroups v2 memory.current | gauge |
| `container.memory_limit_bytes` | cgroups v2 memory.max | gauge |
| `container.pids_current` | cgroups v2 pids.current | gauge |
| `container.network_rx_bytes` | /proc/net/dev inside netns | counter |
| `container.network_tx_bytes` | /proc/net/dev inside netns | counter |
| `container.restart_count` | daemon state | counter |
| `container.uptime_seconds` | daemon state | gauge |

### Lifecycle Events (Logs)

The runtime emits structured JSON logs for lifecycle events:
- `container.created` — container created, image, config
- `container.started` — container started, PID, network
- `container.stopped` — exit code, signal, duration
- `container.oom_killed` — OOM event with memory stats
- `container.health_check` — pass/fail with response time

### Integration Method

1. The Lens agent tails the runtime's structured log files
2. The agent reads cgroup metrics from `/sys/fs/cgroup/<container-id>/`
3. Metrics are tagged with `container_id`, `container_name`, `image`

## Distributed KV Store Instrumentation

### Metrics Exported (via Prometheus /metrics endpoint)

| Metric | Description | Type |
|--------|-------------|------|
| `raft.election_count` | Total leader elections | counter |
| `raft.election_duration_ms` | Time to elect new leader | histogram |
| `raft.replication_lag_entries` | Follower log lag | gauge |
| `raft.commit_index` | Current commit index | gauge |
| `raft.applied_index` | Last applied index | gauge |
| `kv.write_throughput` | Writes per second | gauge |
| `kv.read_throughput` | Reads per second | gauge |
| `kv.compaction_duration_ms` | LSM compaction time | histogram |
| `kv.wal_size_bytes` | Write-ahead log size | gauge |
| `kv.memtable_size_bytes` | MemTable size | gauge |
| `kv.sstable_count` | Number of SSTables | gauge |

### Trace Events

The KV store emits OpenTelemetry spans for:
- Raft leader election (with candidate info)
- Log replication (with entry count)
- Client read/write requests (with latency)
- Snapshot creation and installation
- Membership changes

### Integration Method

1. Lens agent scrapes the KV store's Prometheus `/metrics` endpoint
2. KV store sends OTel traces to the Lens agent on port 4317
3. Structured logs flow into the Lens log pipeline
4. The Watch API config changes trigger Lens alerting rule reloads

## Dashboard Configuration

Dashboard JSON is stored in the KV store at key prefix `/lens/dashboards/`.
The Lens query layer watches this prefix for real-time config updates.

Example dashboard config:
```json
{
  "name": "KV Store Cluster",
  "panels": [
    {"title": "Raft Elections", "query": "raft.election_count", "type": "counter"},
    {"title": "Replication Lag", "query": "max(raft.replication_lag_entries)", "type": "gauge"},
    {"title": "Write Throughput", "query": "sum(kv.write_throughput)", "type": "graph"}
  ],
  "variables": [
    {"name": "node", "query": "label_values(raft.commit_index, node)"}
  ]
}
```
