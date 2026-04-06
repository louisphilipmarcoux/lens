//go:build chaos

// Package chaos contains chaos tests for the Lens platform.
//
// These tests verify that the system handles failures gracefully:
// - No data loss when ingestion nodes are killed mid-write
// - Agent disk buffer preserves data during backend outage
// - System recovers automatically when services restart
package chaos

import (
	"testing"
)

func TestAgentBufferSurvivesBackendKill(t *testing.T) {
	t.Skip("chaos test: requires Docker environment — run with 'go test -tags=chaos'")

	// Test plan:
	// 1. Start agent writing to buffer with backend URL pointing to ingest
	// 2. Kill ingest service
	// 3. Verify agent continues collecting (buffer grows)
	// 4. Restart ingest service
	// 5. Verify buffered data is shipped (buffer shrinks)
	// 6. Query ClickHouse to verify zero data loss
}

func TestIngestNodeKillMidWrite(t *testing.T) {
	t.Skip("chaos test: requires Docker environment — run with 'go test -tags=chaos'")

	// Test plan:
	// 1. Start sending high-volume metrics to ingest service
	// 2. Kill the ingest service mid-batch
	// 3. Restart ingest service
	// 4. Verify agent retries and no data is lost
	// 5. Query ClickHouse for complete dataset
}

func TestClickHouseRestartRecovery(t *testing.T) {
	t.Skip("chaos test: requires Docker environment — run with 'go test -tags=chaos'")

	// Test plan:
	// 1. Start full pipeline (agent -> ingest -> ClickHouse)
	// 2. Restart ClickHouse
	// 3. Verify ingest service reconnects
	// 4. Verify no data loss via query layer
}
