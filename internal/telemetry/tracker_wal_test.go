//go:build !lite

package telemetry

import (
	"context"
	"testing"
)

// TestTrackerDatabaseUsesWAL pins the pragma the tracker's DSN used to leave
// out. Without journal_mode(WAL) the database runs in rollback-journal mode,
// where a writer blocks every reader for the whole transaction: the tracker is
// written from a background goroutine while the CLI reads the summary for the
// same process, which is exactly the pattern the pool discipline in AGENTS.md
// exists to keep cheap.
func TestTrackerDatabaseUsesWAL(t *testing.T) {
	tracker := newTestTracker(t)
	tracker.Wait()

	if tracker.db == nil {
		t.Fatal("tracker database was never opened")
	}

	var journalMode string
	if err := tracker.db.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var synchronous int
	if err := tracker.db.QueryRowContext(context.Background(), "PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if synchronous != 1 {
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}
}
