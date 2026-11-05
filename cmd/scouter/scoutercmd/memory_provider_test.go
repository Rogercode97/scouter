package scoutercmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMemoryProvider_AbsentDatabaseDisablesHistory pins the graceful path: no
// observation database means nil, and the engines score without a multiplier.
func TestMemoryProvider_AbsentDatabaseDisablesHistory(t *testing.T) {
	t.Setenv("ENGRAM_DB_PATH", filepath.Join(t.TempDir(), "absent.db"))

	if p := memoryProvider(); p != nil {
		t.Fatal("expected nil provider when the database is absent")
	}
}

// TestMemoryProvider_PresentDatabaseEnablesHistory is the wiring guard for the
// historical fragility multiplier: every surface must hand the engines a real
// provider when the database exists, or RiskScore silently loses that factor.
func TestMemoryProvider_PresentDatabaseEnablesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("create database file: %v", err)
	}
	t.Setenv("ENGRAM_DB_PATH", path)

	if p := memoryProvider(); p == nil {
		t.Fatal("expected a provider when the database exists")
	}
}
