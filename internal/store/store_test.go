package store

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestStoreSearch(t *testing.T) {
	ctx := t.Context()
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
	ctx := t.Context()
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

func TestGetImpact(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_impact.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Satisfy FKs
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "a.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "b.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "c.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "d.go", Project: "p"})

	s.SaveSymbol(ctx, &Symbol{Name: "A", Type: "func", Path: "a.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "B", Type: "func", Path: "b.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "C", Type: "func", Path: "c.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "D", Type: "func", Path: "d.go", StartByte: 0, EndByte: 10})

	// 1. Build Graph: A -> B, B -> C, C -> A (cycle), D -> B
	calls := []Call{
		{CallerName: "A", CalleeName: "B", Path: "a.go", CalleePath: "b.go"},
		{CallerName: "B", CalleeName: "C", Path: "b.go", CalleePath: "c.go"},
		{CallerName: "C", CalleeName: "A", Path: "c.go", CalleePath: "a.go"},
		{CallerName: "D", CalleeName: "B", Path: "d.go", CalleePath: "b.go"},
	}
	for _, c := range calls {
		if err := s.SaveCall(ctx, c); err != nil {
			t.Fatalf("Failed to save call in test graph: %v", err)
		}
	}
	s.ResolveCentrality(ctx)

	// 2. Impact of B
	impact, err := s.GetImpact(ctx, "B", "b.go", 3)
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}

	foundA, foundD, foundC, foundB := false, false, false, false
	for _, r := range impact.Callers {
		switch r.Symbol {
		case "A": foundA = true
		case "D": foundD = true
		case "C": foundC = true
		case "B": foundB = true
		}
	}

	if !foundA || !foundD || !foundC || !foundB {
		t.Errorf("Incomplete impact results: %+v", impact.Callers)
	}

	if !impact.Target.Metrics.PublicExport {
		t.Errorf("Expected B to be public export")
	}
	if impact.RiskLevel != "Medium" {
		t.Errorf("Expected RiskLevel 'Medium', got %s", impact.RiskLevel)
	}
	if impact.Target.RiskScore < 0.2 || impact.Target.RiskScore >= 0.5 {
		t.Errorf("Risk score %v outside of expected bounds", impact.Target.RiskScore)
	}
	if impact.Mermaid == "" || impact.Mermaid[:8] != "graph TD" {
		t.Errorf("Expected valid Mermaid graph, got: %s", impact.Mermaid)
	}

	impact2, _ := s.GetImpact(ctx, "B", "b.go", 1)
	if len(impact2.Callers) != 2 {
		t.Errorf("Depth capping failed, expected 2 results, got %d: %+v", len(impact2.Callers), impact2)
	}
}
func TestGetUnusedSymbols(t *testing.T) {
	ctx := t.Context()
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
	unused, err := s.GetUnusedSymbols(ctx, false)
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
	unused, err = s.GetUnusedSymbols(ctx, true)
	if err != nil {
		t.Fatalf("GetUnusedSymbols failed: %v", err)
	}

	if len(unused) != 2 {
		t.Errorf("Expected 2 unused symbols, got %d", len(unused))
	}
}

func TestStoreSearch_Injection(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_injection.db"
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

	syms := []Symbol{
		{Name: "NormalSymbol", Type: "method", Path: "store.go", StartByte: 100, EndByte: 200},
		{Name: "Special+Symbol-With:Chars", Type: "function", Path: "store.go", StartByte: 10, EndByte: 50},
	}

	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// Try an injection attempt
	_, err = s.SearchSymbols(ctx, "Normal\" OR 1=1 --", "")
	if err != nil {
		t.Errorf("Injection search failed (syntax error?): %v", err)
	}
}

func TestSaveFileIndex_PreservesSymbols(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_preserve.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path := "preserve.go"
	// 1. Save file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    path,
		Mtime:   100,
		Hash:    "h1",
		ASTJSON: "{}",
		Project: "p1",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// 2. Save a symbol linked to that file
	sym := &Symbol{Name: "KeepMe", Type: "func", Path: path, StartByte: 0, EndByte: 10}
	if err := s.SaveSymbol(ctx, sym); err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	// 3. Update file index (same path)
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    path,
		Mtime:   200, // changed
		Hash:    "h2",  // changed
		ASTJSON: "{}",
		Project: "p1",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex update failed: %v", err)
	}

	// 4. Verify symbol still exists
	res, err := s.SearchSymbols(ctx, "KeepMe", "")
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}

	if len(res) == 0 {
		t.Error("Symbol was DELETED during FileIndex update (unintended cascade delete). INSERT OR REPLACE is likely the cause.")
	}
}

func TestStore_DeleteCascade(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_cascade.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path := "cascade.go"
	// 1. Save file index
	err = s.SaveFileIndex(ctx, &FileIndex{Path: path, Project: "p1"})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// 2. Save a symbol linked to that file
	sym := &Symbol{Name: "DeleteMe", Type: "func", Path: path}
	if err := s.SaveSymbol(ctx, sym); err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	// 3. Delete file index
	if err := s.DeleteFileIndex(ctx, path); err != nil {
		t.Fatalf("DeleteFileIndex failed: %v", err)
	}

	// 4. Verify symbol is GONE (cascade delete)
	res, err := s.SearchSymbols(ctx, "DeleteMe", "")
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}

	if len(res) != 0 {
		t.Error("Symbol was NOT deleted when file_index was deleted (foreign keys might be OFF)")
	}
}

func TestStore_HasColumn(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_hascolumn.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	storeImpl := s.(*Store)
	tx, _ := storeImpl.db.BeginTx(ctx, nil)
	defer tx.Rollback()

	// 1. Check existing column
	has, err := hasColumn(ctx, tx, "symbols", "name")
	if err != nil {
		t.Fatalf("hasColumn(symbols, name) failed: %v", err)
	}
	if !has {
		t.Error("Expected hasColumn(symbols, name) to be true")
	}

	// 2. Check non-existing column
	has, err = hasColumn(ctx, tx, "symbols", "nonexistent")
	if err != nil {
		t.Fatalf("hasColumn(symbols, nonexistent) failed: %v", err)
	}
	if has {
		t.Error("Expected hasColumn(symbols, nonexistent) to be false")
	}

	// 3. Check existing table but invalid column (handled above)
	
	// 4. Check non-existing table
	has, err = hasColumn(ctx, tx, "nonexistent_table", "doc")
	if err != nil {
		t.Fatalf("hasColumn(nonexistent_table, doc) failed: %v", err)
	}
	if has {
		t.Error("Expected hasColumn(nonexistent_table, doc) to be false")
	}
}

func TestSanitizeFTS_SpecialCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"*", "\"\"*"},
		{"normal", "\"normal\""},
		{"prefix*", "\"prefix\"*"},
		{"with+plus", "\"with+plus\""},
		{"with-minus", "\"with-minus\""},
		{"with:colon", "\"with:colon\""},
		{"with^caret", "\"with^caret\""},
		{"AND", "\"AND\""},
		{"OR", "\"OR\""},
		{"NOT", "\"NOT\""},
		{"NEAR", "\"NEAR\""},
		{"\"quotes\"", "\"\"\"quotes\"\"\""},
		{"a*b", "\"a*b\""},
		{"*", "\"\"*"},
	}

	for _, tt := range tests {
		actual := sanitizeFTS(tt.input)
		if actual != tt.expected {
			t.Errorf("sanitizeFTS(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestStore_TransactionSafety(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_tx.db"
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

	// Try a transaction that fails halfway
	err = s.WithTransaction(ctx, func(txCtx context.Context, tx Repository) error {
		sym := &Symbol{Name: "PartiallySaved", Type: "func", Path: "store.go"}
		if err := tx.SaveSymbol(txCtx, sym); err != nil {
			return err
		}
		return fmt.Errorf("simulated error")
	})

	if err == nil || err.Error() != "simulated error" {
		t.Errorf("Expected simulated error, got %v", err)
	}

	// Verify the symbol was NOT saved
	results, _ := s.SearchSymbols(ctx, "PartiallySaved", "")
	if len(results) != 0 {
		t.Errorf("Expected 0 results after rollback, got %d", len(results))
	}
	}

	func TestGetSymbolsByNameInFile(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_namefile.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path1 := "file1.go"
	path2 := "file2.go"
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: path1, Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: path2, Project: "p"})

	sym1 := Symbol{Name: "MySym", Type: "func", Path: path1, StartByte: 10, EndByte: 20}
	sym2 := Symbol{Name: "MySym", Type: "func", Path: path2, StartByte: 30, EndByte: 40}
	_ = s.SaveSymbol(ctx, &sym1)
	_ = s.SaveSymbol(ctx, &sym2)

	res, err := s.GetSymbolsByNameInFile(ctx, "MySym", path1)
	if err != nil {
		t.Fatalf("GetSymbolsByNameInFile failed: %v", err)
	}

	if len(res) != 1 {
		t.Errorf("Expected 1 result, got %d", len(res))
	} else if res[0].Path != path1 {
		t.Errorf("Expected path %s, got %s", path1, res[0].Path)
	}
}

func TestGetSymbolsByType(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_type.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "types.go", Project: "p"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape", Type: "interface", Path: "types.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle", Type: "struct", Path: "types.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Area", Type: "method", Path: "types.go"})

	res, err := s.GetSymbolsByType(ctx, "interface")
	if err != nil {
		t.Fatalf("GetSymbolsByType failed: %v", err)
	}

	if len(res) != 1 || res[0].Name != "Shape" {
		t.Errorf("Expected 1 interface (Shape), got %v", res)
	}
}

func TestCallLinkTypePersistence(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_linktype.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "main.go", Project: "p"})

	// Test default link type
	_ = s.SaveCall(ctx, Call{CallerName: "A", CalleeName: "B", Path: "main.go"})
	// Test dynamic link type
	_ = s.SaveCall(ctx, Call{CallerName: "Iface.M", CalleeName: "Impl.M", Path: "main.go", LinkType: "dynamic"})

	callers, _ := s.GetCallers(ctx, "Impl.M")
	if len(callers) != 1 || callers[0].LinkType != "dynamic" {
		t.Errorf("Expected dynamic link type, got %v", callers)
	}
}