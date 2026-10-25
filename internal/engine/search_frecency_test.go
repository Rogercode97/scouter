package engine

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

// TestSearchSymbols_FrecencyOrder pins the frecency contract: among symbols
// matching the same FTS query, the one with more runtime_hits ranks first.
// Ties keep insertion order so the ranking is deterministic.
func TestSearchSymbols_FrecencyOrder(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	seed := func(name string) {
		t.Helper()
		if err := s.SaveSymbol(ctx, &store.Symbol{Name: name, Type: "function", Path: "/tmp/freq/" + name + ".go", Doc: "frecency probe ledger"}); err != nil {
			t.Fatalf("SaveSymbol %s failed: %v", name, err)
		}
	}
	seed("freq_cold")
	seed("freq_hot")

	if err := s.UpdateSymbolRuntimeHits(ctx, "freq_hot", "/tmp/freq/freq_hot.go", 5); err != nil {
		t.Fatalf("UpdateSymbolRuntimeHits failed: %v", err)
	}

	res, err := s.SearchSymbols(ctx, "frecency probe", "", "", 10, 0)
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(res))
	}
	if res[0].Name != "freq_hot" {
		t.Errorf("expected freq_hot first (5 hits), got %q", res[0].Name)
	}
}

// TestHybridSearch_BumpsRuntimeHits pins the learning half: a successful
// search records one hit per returned symbol so later searches rank them
// higher. The bump is best-effort and never appears in the results.
func TestHybridSearch_BumpsRuntimeHits(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	if err := s.SaveSymbol(ctx, &store.Symbol{Name: "freq_seen", Type: "function", Path: "/tmp/freq/seen.go", Doc: "frecency probe ledger"}); err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	e := NewSearchEngine(s, nil, nil)
	res, err := e.HybridSearch(ctx, "frecency probe", 10, 0, "")
	if err != nil {
		t.Fatalf("HybridSearch failed: %v", err)
	}
	if len(res.Symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(res.Symbols))
	}

	got, err := s.GetSymbolsByNameInFile(ctx, "freq_seen", "/tmp/freq/seen.go")
	if err != nil || len(got) == 0 {
		t.Fatalf("GetSymbolsByNameInFile failed: %v", err)
	}
	if got[0].RuntimeHits != 1 {
		t.Errorf("expected 1 runtime hit after one search, got %d", got[0].RuntimeHits)
	}
}
