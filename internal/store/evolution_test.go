package store

import (
	"context"
	"os"
	"testing"
)

func TestEvolution(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_evolution.db"
	defer os.Remove(dbPath)

	s, err := NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	paths := []string{"file1.go", "file2.ts", "file3.py"}
	for _, p := range paths {
		err = s.SaveFileIndex(ctx, &FileIndex{
			Path:    p,
			Mtime:   12345,
			Hash:    "hash",
			AstJson: "{}",
			Project: "scouter",
		})
		if err != nil {
			t.Fatalf("Failed to save file index for %s: %v", p, err)
		}

		// Save a symbol and call to test CASCADE
		err = s.SaveSymbol(ctx, &Symbol{Name: "func_" + p, Path: p})
		if err != nil {
			t.Fatalf("Failed to save symbol for %s: %v", p, err)
		}
		err = s.SaveCall(ctx, Call{CallerName: "main", CalleeName: "func_" + p, Path: p, Line: 1})
		if err != nil {
			t.Fatalf("Failed to save call for %s: %v", p, err)
		}
	}

	// Test GetAllFilePaths
	dbPaths, err := s.GetAllFilePaths(ctx)
	if err != nil {
		t.Fatalf("GetAllFilePaths failed: %v", err)
	}
	if len(dbPaths) != 3 {
		t.Errorf("Expected 3 paths, got %d", len(dbPaths))
	}

	// Test DeleteFileIndex
	err = s.DeleteFileIndex(ctx, "file1.go")
	if err != nil {
		t.Fatalf("DeleteFileIndex failed: %v", err)
	}

	dbPaths, _ = s.GetAllFilePaths(ctx)
	if len(dbPaths) != 2 {
		t.Errorf("Expected 2 paths after deletion, got %d", len(dbPaths))
	}

	found := false
	for _, p := range dbPaths {
		if p == "file1.go" {
			found = true
		}
	}
	if found {
		t.Error("file1.go still exists after deletion")
	}

	// Verify CASCADE (symbols and calls should be gone for file1.go)
	var count int
	err = s.(*storeImpl).db.QueryRow("SELECT COUNT(*) FROM symbols WHERE path = 'file1.go'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query symbols: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 symbols for file1.go due to CASCADE, got %d", count)
	}

	err = s.(*storeImpl).db.QueryRow("SELECT COUNT(*) FROM calls WHERE path = 'file1.go'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query calls: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 calls for file1.go due to CASCADE, got %d", count)
	}
}
