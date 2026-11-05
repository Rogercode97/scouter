package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportDeltaSurfacesCallSaveFailures pins the difference between "the
// delta was applied" and "the delta was walked". A call whose path is absent
// from file_index violates the foreign key; the insert is OR IGNORE, which
// suppresses uniqueness conflicts but not foreign-key ones, so the statement
// fails. ImportDelta used to drop that error on the floor and commit a
// half-applied file, reporting success with the call edges missing.
func TestImportDeltaSurfacesCallSaveFailures(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(ctx, filepath.Join(t.TempDir(), "import_delta.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	syncDir := filepath.Join(t.TempDir(), "sync")
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatalf("sync dir: %v", err)
	}

	delta := SovereignDelta{
		Path: "alpha.go",
		Hash: "h-alpha",
		Symbols: []Symbol{
			{Name: "Alpha", Type: "function", Path: "alpha.go", StartLine: 1, EndLine: 2},
		},
		Calls: []Call{
			{CallerName: "Alpha", CalleeName: "Ghost", Path: "ghost.go", Line: 7},
		},
	}
	raw, err := json.Marshal(delta)
	if err != nil {
		t.Fatalf("marshal delta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(syncDir, "alpha.json"), raw, 0o644); err != nil {
		t.Fatalf("write delta: %v", err)
	}

	err = s.ImportDelta(ctx, syncDir)
	if err == nil {
		t.Fatal("RED: ImportDelta reported success while the call insert violated the file_index foreign key")
	}
	if !strings.Contains(err.Error(), "failed to save call") {
		t.Fatalf("error does not name the failing operation: %v", err)
	}
}
