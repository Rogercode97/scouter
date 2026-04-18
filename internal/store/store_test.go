package store

import (
	"context"
	"os"
	"testing"
)

func TestStoreSearch(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_scouter.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index to satisfy foreign key
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "store.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save dummy symbols
	syms := []Symbol{
		{Name: "SearchSymbols", Type: "method", Path: "store.go", StartByte: 100, EndByte: 200},
		{Name: "New", Type: "function", Path: "store.go", StartByte: 10, EndByte: 50},
		{Name: "Store", Type: "class", Path: "store.go", StartByte: 0, EndByte: 5},
	}

	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// 2. Test search
	results, err := s.SearchSymbols(ctx, "Search*", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result for 'Search*'")
	}

	found := false
	for _, r := range results {
		if r.Name == "SearchSymbols" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Did not find 'SearchSymbols' in results")
	}

	// 3. Test filter by type
	results, _ = s.SearchSymbols(ctx, "Store", "class")
	if len(results) != 1 || results[0].Type != "class" {
		t.Errorf("Expected 1 class result, got %d", len(results))
	}
}
