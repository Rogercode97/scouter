package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	driversqlite "modernc.org/sqlite"
)

// TestAnnotateSQLError_CodeAndNoLeak pins the Engram-ported error contract:
// driver failures carry the operation and the numeric SQLite result code,
// keep the %w chain for errors.As, and never render SQL text, arguments,
// DSN or filesystem paths.
func TestAnnotateSQLError_CodeAndNoLeak(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "CREATE TABLE probe (a TEXT UNIQUE)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO probe VALUES ('x')"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	const dupSQL = "INSERT INTO probe VALUES ('x')"
	_, driverErr := db.ExecContext(ctx, dupSQL)
	if driverErr == nil {
		t.Fatal("expected duplicate-key error, got nil")
	}

	annotated := annotateSQLError("insert probe", driverErr)
	if annotated == nil {
		t.Fatal("expected annotated error, got nil")
	}
	msg := annotated.Error()
	if !strings.Contains(msg, "insert probe") {
		t.Errorf("message lost the operation: %q", msg)
	}
	// modernc reports the extended code: 2067 is SQLITE_CONSTRAINT_UNIQUE
	// (base SQLITE_CONSTRAINT 19 | 8<<8), not the bare primary code.
	if !strings.Contains(msg, "code 2067") {
		t.Errorf("message lost the numeric code: %q", msg)
	}
	if strings.Contains(msg, dupSQL) {
		t.Errorf("message leaks SQL text: %q", msg)
	}
	var serr *driversqlite.Error
	if !errors.As(annotated, &serr) {
		t.Errorf("chain lost *sqlite.Error for errors.As: %q", msg)
	}
	if !errors.Is(annotated, driverErr) {
		t.Errorf("chain lost the original error: %q", msg)
	}
}
