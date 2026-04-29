package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// SearchEngine unifies AST structural search with Engram historical insights.
type SearchEngine struct {
	store store.Repository
}

func NewSearchEngine(s store.Repository) *SearchEngine {
	return &SearchEngine{store: s}
}

// HybridSearch executes parallel lookups in the AST and Engram databases.
func (e *SearchEngine) HybridSearch(ctx context.Context, query string) (*types.HybridSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("missing query")
	}

	type symRes struct {
		symbols []store.Symbol
		err     error
	}
	type insRes struct {
		insights []types.MemoryInsight
		err      error
	}

	symChan := make(chan symRes, 1)
	insChan := make(chan insRes, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		res, err := e.store.SearchSymbols(ctx, query, "")
		symChan <- symRes{res, err}
	}()

	go func() {
		defer wg.Done()
		res, err := e.store.GetMemoryInsights(ctx, query)
		insChan <- insRes{res, err}
	}()

	wg.Wait()
	close(symChan)
	close(insChan)

	sRes := <-symChan
	if sRes.err != nil {
		return nil, fmt.Errorf("AST search failed: %w", sRes.err)
	}

	iRes := <-insChan
	if iRes.err != nil {
		return nil, fmt.Errorf("Engram search failed: %w", iRes.err)
	}

	// Map store.Symbol to types.Symbol (Sovereign Mapping)
	var symbols []types.Symbol
	for _, s := range sRes.symbols {
		symbols = append(symbols, types.Symbol{
			Name:      s.Name,
			Type:      s.Type,
			Signature: s.Signature,
			Doc:       s.Doc,
			Path:      s.Path,
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
		})
	}

	return &types.HybridSearchResult{
		Symbols:  symbols,
		Insights: iRes.insights,
	}, nil
}
