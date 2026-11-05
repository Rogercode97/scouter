package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestClearSemanticVectorsRemovesOrphans(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "vec_orphan.db")
	s, err := NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	impl, ok := s.(*storeImpl)
	if !ok {
		t.Fatalf("store is %T, want *storeImpl", s)
	}
	countVec := func() int {
		var n int
		if err := impl.queryRow(ctx, "SELECT COUNT(*) FROM vec_symbols").Scan(&n); err != nil {
			t.Fatalf("count vec: %v", err)
		}
		return n
	}

	first := []BatchItem{{
		Index:   &FileIndex{Path: "orphan.go", Mtime: 1, Hash: "h1", AstJson: "{}", Project: "t"},
		Symbols: []Symbol{{Name: "Orphan", Type: "function", Path: "orphan.go", StartLine: 1, EndLine: 2}},
	}}
	if err := s.SaveFileIndexBatch(ctx, first); err != nil {
		t.Fatalf("batch1: %v", err)
	}
	syms, err := s.GetSymbolsByNameInFile(ctx, "Orphan", "orphan.go")
	if err != nil || len(syms) == 0 {
		t.Fatalf("seed symbol missing: %v", err)
	}
	var realID int
	if err := impl.queryRow(ctx, "SELECT id FROM symbols WHERE name = ? AND path = ?", "Orphan", "orphan.go").Scan(&realID); err != nil {
		t.Fatalf("seed id: %v", err)
	}
	vec := make([]float32, 384)
	vec[0] = 1
	if err := s.InsertSemanticVector(ctx, int64(realID), vec); err != nil {
		t.Fatalf("insert vec: %v", err)
	}
	if n := countVec(); n != 1 {
		t.Fatalf("vec rows before reindex = %d, want 1", n)
	}

	second := []BatchItem{{
		Index:   &FileIndex{Path: "orphan.go", Mtime: 2, Hash: "h2", AstJson: "{}", Project: "t"},
		Symbols: []Symbol{{Name: "Orphan2", Type: "function", Path: "orphan.go", StartLine: 1, EndLine: 2}},
	}}
	if err := s.SaveFileIndexBatch(ctx, second); err != nil {
		t.Fatalf("batch2: %v", err)
	}
	if n := countVec(); n != 0 {
		t.Fatalf("RED: vec orphans after reindex = %d, want 0", n)
	}
}

// TestClearSymbolsRemovesSemanticVectors pins the invariant at the function
// every caller already uses. An engine that removes a path's symbols with
// ClearSymbols alone (the healer, index-vault) used to leave vec_symbols rows
// pointing at ids that no longer exist: the vec table has no cascade, and the
// nearest-neighbour read path spends its k budget on those orphans.
func TestClearSymbolsRemovesSemanticVectors(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(ctx, filepath.Join(t.TempDir(), "clear_symbols.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	impl, ok := s.(*storeImpl)
	if !ok {
		t.Fatalf("store is %T, want *storeImpl", s)
	}
	countVec := func() int {
		var n int
		if err := impl.queryRow(ctx, "SELECT COUNT(*) FROM vec_symbols").Scan(&n); err != nil {
			t.Fatalf("count vec: %v", err)
		}
		return n
	}
	countSymbols := func() int {
		var n int
		if err := impl.queryRow(ctx, `SELECT COUNT(*) FROM symbols WHERE path = 'healed.go'`).Scan(&n); err != nil {
			t.Fatalf("count symbols: %v", err)
		}
		return n
	}

	seed := []BatchItem{{
		Index:   &FileIndex{Path: "healed.go", Mtime: 1, Hash: "h1", AstJson: "{}", Project: "t"},
		Symbols: []Symbol{{Name: "Healed", Type: "function", Path: "healed.go", StartLine: 1, EndLine: 2}},
	}}
	if err := s.SaveFileIndexBatch(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, name := range []string{"healed.go", "neighbour.go"} {
		if name != "healed.go" {
			other := []BatchItem{{
				Index:   &FileIndex{Path: name, Mtime: 1, Hash: "h2", AstJson: "{}", Project: "t"},
				Symbols: []Symbol{{Name: "Neighbour", Type: "function", Path: name, StartLine: 1, EndLine: 2}},
			}}
			if err := s.SaveFileIndexBatch(ctx, other); err != nil {
				t.Fatalf("seed neighbour: %v", err)
			}
		}
	}

	var healedID, neighbourID int
	if err := impl.queryRow(ctx, "SELECT id FROM symbols WHERE name = 'Healed'").Scan(&healedID); err != nil {
		t.Fatalf("seed id: %v", err)
	}
	if err := impl.queryRow(ctx, "SELECT id FROM symbols WHERE name = 'Neighbour'").Scan(&neighbourID); err != nil {
		t.Fatalf("neighbour id: %v", err)
	}
	vec := make([]float32, 384)
	vec[0] = 1
	if err := s.InsertSemanticVector(ctx, int64(healedID), vec); err != nil {
		t.Fatalf("insert healed vec: %v", err)
	}
	if err := s.InsertSemanticVector(ctx, int64(neighbourID), vec); err != nil {
		t.Fatalf("insert neighbour vec: %v", err)
	}
	if n := countVec(); n != 2 {
		t.Fatalf("vec rows before clear = %d, want 2", n)
	}

	if err := s.ClearSymbols(ctx, "healed.go"); err != nil {
		t.Fatalf("clear symbols: %v", err)
	}

	if n := countSymbols(); n != 0 {
		t.Fatalf("symbols left for cleared path = %d, want 0", n)
	}
	if n := countVec(); n != 1 {
		t.Fatalf("RED: vec orphans after ClearSymbols = %d, want 1 (only the neighbour's row)", n)
	}

	if err := s.ClearSymbols(ctx, "healed.go"); err != nil {
		t.Fatalf("clear symbols twice: %v", err)
	}
	if n := countVec(); n != 1 {
		t.Fatalf("vec rows after repeated clear = %d, want 1", n)
	}
}
