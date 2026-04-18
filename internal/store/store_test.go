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

func TestStoreCalls(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_scouter_calls.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "main.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save dummy calls
	calls := []Call{
		{CallerName: "main", CalleeName: "foo", Path: "main.go", Line: 10},
		{CallerName: "foo", CalleeName: "bar", Path: "main.go", Line: 20},
		{CallerName: "main", CalleeName: "bar", Path: "main.go", Line: 15},
	}

	for _, c := range calls {
		if err := s.SaveCall(ctx, c); err != nil {
			t.Fatalf("Failed to save call from %s to %s: %v", c.CallerName, c.CalleeName, err)
		}
	}

	// 2. Test GetCallers
	callers, err := s.GetCallers(ctx, "bar")
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	if len(callers) != 2 {
		t.Errorf("Expected 2 callers for 'bar', got %d", len(callers))
	}

	// 3. Test ClearCalls
	if err := s.ClearCalls(ctx, "main.go"); err != nil {
		t.Fatalf("ClearCalls failed: %v", err)
	}

	callers, _ = s.GetCallers(ctx, "bar")
	if len(callers) != 0 {
		t.Errorf("Expected 0 callers after ClearCalls, got %d", len(callers))
	}
}

func TestGetUnusedSymbols(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_scouter_deadcode.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "logic.go",
		Mtime:   123456789,
		Hash:    "hash1",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save symbols: 
	// - "usedFunc": will have a caller
	// - "deadFunc": no callers
	// - "ExportedUnused": exported, no callers
	syms := []Symbol{
		{Name: "usedFunc", Type: "function", Path: "logic.go", StartByte: 10, EndByte: 20},
		{Name: "deadFunc", Type: "function", Path: "logic.go", StartByte: 30, EndByte: 40},
		{Name: "ExportedUnused", Type: "function", Path: "logic.go", StartByte: 50, EndByte: 60},
	}
	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// 2. Save call to "usedFunc"
	err = s.SaveCall(ctx, Call{CallerName: "main", CalleeName: "usedFunc", Path: "logic.go", Line: 100})
	if err != nil {
		t.Fatalf("Failed to save call: %v", err)
	}

	// 3. Test GetUnusedSymbols (IncludeExported = false)
	// Should only find "deadFunc"
	unused, err := s.GetUnusedSymbols(ctx, DeadCodeOptions{IncludeExported: false})
	if err != nil {
		t.Fatalf("GetUnusedSymbols failed: %v", err)
	}

	if len(unused) != 1 {
		t.Errorf("Expected 1 unused symbol, got %d", len(unused))
	} else if unused[0].Name != "deadFunc" {
		t.Errorf("Expected 'deadFunc' to be unused, got '%s'", unused[0].Name)
	}

	// 4. Test GetUnusedSymbols (IncludeExported = true)
	// Should find "deadFunc" and "ExportedUnused"
	unused, err = s.GetUnusedSymbols(ctx, DeadCodeOptions{IncludeExported: true})
	if err != nil {
		t.Fatalf("GetUnusedSymbols failed: %v", err)
	}

	if len(unused) != 2 {
		t.Errorf("Expected 2 unused symbols, got %d", len(unused))
	}
}
