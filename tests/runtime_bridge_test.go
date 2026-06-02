package tests

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/telemetry/ingest"
	_ "modernc.org/sqlite"
)

func TestRuntimeBridgeE2E(t *testing.T) {
	// Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "scouter.db")
	ctx := context.Background()

	// Initialize the Store (this creates the DB and runs migrations)
	dbStore, err := store.NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer dbStore.Close()

	// Insert FileIndex first to satisfy foreign key constraint
	err = dbStore.SaveFileIndex(ctx, &store.FileIndex{
		Path:    "pkg/target.go",
		Hash:    "dummyhash",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// Insert a dummy symbol to map to
	err = dbStore.SaveSymbol(ctx, &store.Symbol{
		Name:      "TestTargetFunc",
		Type:      "function",
		Path:      "pkg/target.go",
		StartLine: 10,
		EndLine:   20,
	})
	if err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	// Prepare JSON Lines payload
	jsonLines := []byte(`
{"trace_id":"trace1","span_id":"span1","symbol_name":"TestTargetFunc","symbol_path":"pkg/target.go","timestamp":"2023-10-01T12:00:00Z"}
{"trace_id":"trace2","span_id":"span2","symbol_name":"TestTargetFunc","symbol_path":"pkg/target.go","timestamp":"2023-10-01T12:01:00Z"}
{"trace_id":"trace3","span_id":"span3","symbol_name":"UnmappedFunc","symbol_path":"pkg/other.go","timestamp":"2023-10-01T12:02:00Z"}
{"invalid_json" : true
`)
	reader := bytes.NewReader(jsonLines)

	// Run Ingest
	err = ingest.Ingest(ctx, reader, "staging", dbStore)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Wait for any async routines if there are any (Ingest is synchronous based on phase 2 though)

	// Verify the usage record was stored for TestTargetFunc by connecting directly with sql.DB
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer sqlDB.Close()

	var env string
	var count int
	// We join with symbols to get the name for verification
	err = sqlDB.QueryRowContext(ctx, `
		SELECT u.environment, u.hit_count 
		FROM symbol_usage u
		JOIN symbols s ON u.symbol_id = s.id
		WHERE s.name = 'TestTargetFunc' AND s.path = 'pkg/target.go'
	`).Scan(&env, &count)

	if err != nil {
		t.Fatalf("Failed to query symbol_usage: %v", err)
	}

	if env != "staging" {
		t.Errorf("Expected environment 'staging', got %s", env)
	}
	if count != 2 {
		t.Errorf("Expected hit_count 2, got %d", count)
	}

	// Verify UnmappedFunc wasn't added
	var unmappedCount int
	err = sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols WHERE name = 'UnmappedFunc'").Scan(&unmappedCount)
	if err != nil {
		t.Fatalf("Failed to count UnmappedFunc: %v", err)
	}
	if unmappedCount != 0 {
		t.Errorf("Expected 0 UnmappedFunc symbols, got %d", unmappedCount)
	}
}