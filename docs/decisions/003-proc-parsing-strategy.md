# ADR-003: /proc Parsing Strategy

## Status

Accepted

## Context

The agent collects host metrics by parsing files in `/proc`. This raises two issues:

1. **Testability**: `/proc` only exists on Linux. The agent is developed on Windows and macOS and tested in CI on Linux. Tests must work on all platforms.
2. **Parsing approach**: We could use existing libraries (e.g., `prometheus/procfs`) or write custom parsers.

## Decision

All `/proc` parsers accept an `io.Reader` rather than opening files directly. A thin wrapper opens the actual file at the call site. Tests use `strings.NewReader()` or `os.Open("testdata/...")` with snapshot files copied from a real Linux system.

We write custom parsers rather than depending on `prometheus/procfs` because:
- We only need a subset of the data (CPU, memory, disk, network, loadavg)
- Custom parsers give us full control over the output format and metric naming
- Fewer dependencies for a portfolio project where understanding the internals is the point

## Consequences

- **Cross-platform testing**: All parser unit tests run on Windows, macOS, and Linux without modification.
- **Testdata maintenance**: The `testdata/` directory contains static snapshots. These need to be updated if the parser needs to handle new `/proc` formats, but `/proc` formats are extremely stable.
- **Integration testing**: Full agent integration tests still require Linux (Docker in CI) to validate the file-opening layer.
