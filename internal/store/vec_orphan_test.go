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
