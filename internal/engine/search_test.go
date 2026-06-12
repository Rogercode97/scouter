package engine

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

type searchTestSymbolRegistry struct {
	store.SymbolRegistry
	SearchSymbolsFunc func(ctx context.Context, query, kind string, limit, offset int) ([]store.Symbol, error)
	SearchHybridFunc  func(ctx context.Context, textQuery string, queryEmbedding []float32, limit int) ([]store.Symbol, error)
}

func (m *searchTestSymbolRegistry) SearchSymbols(ctx context.Context, query, kind string, limit, offset int) ([]store.Symbol, error) {
	if m.SearchSymbolsFunc != nil {
		return m.SearchSymbolsFunc(ctx, query, kind, limit, offset)
	}
	return nil, nil
}

func (m *searchTestSymbolRegistry) SearchHybrid(ctx context.Context, textQuery string, queryEmbedding []float32, limit int) ([]store.Symbol, error) {
	if m.SearchHybridFunc != nil {
		return m.SearchHybridFunc(ctx, textQuery, queryEmbedding, limit)
	}
	return nil, nil
}

func TestHybridSearch_FallbackToFTS5_WhenSemanticEngineNil(t *testing.T) {
	mockStore := &searchTestSymbolRegistry{
		SearchSymbolsFunc: func(ctx context.Context, query, kind string, limit, offset int) ([]store.Symbol, error) {
			return []store.Symbol{
				{Name: "TestSymbol"},
			}, nil
		},
	}

	searchEngine := NewSearchEngine(mockStore, nil, nil)
	
	res, err := searchEngine.HybridSearch(context.Background(), "test", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Symbols) != 1 || res.Symbols[0].Name != "TestSymbol" {
		t.Errorf("expected 1 symbol 'TestSymbol', got %+v", res.Symbols)
	}
}

func TestHybridSearch_FailsIfQueryEmpty(t *testing.T) {
	searchEngine := NewSearchEngine(nil, nil, nil)
	_, err := searchEngine.HybridSearch(context.Background(), "", 10, 0)
	if err == nil {
		t.Error("expected error for empty query")
	}
}
