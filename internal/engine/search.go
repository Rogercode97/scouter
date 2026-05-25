package engine

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// SearchEngine unifies AST structural search with Engram historical insights.
type SearchEngine struct {
	store  store.SymbolRegistry
	memory memory.MemoryProvider
}

func NewSearchEngine(s store.SymbolRegistry, m memory.MemoryProvider) *SearchEngine {
	return &SearchEngine{store: s, memory: m}
}

// HybridSearch executes parallel lookups in the AST and Engram databases.
func (e *SearchEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {
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
		res, err := e.store.SearchSymbols(ctx, query, "", limit, offset)
		symChan <- symRes{res, err}
	}()

	go func() {
		defer wg.Done()
		if e.memory == nil {
			insChan <- insRes{nil, nil}
			return
		}
		res, err := e.memory.SearchInsights(ctx, query, 5)
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
			Name:         s.Name,
			Type:         s.Type,
			PackagePath:  s.PackagePath,
			ReceiverType: s.ReceiverType,
			Signature:    s.Signature,
			Doc:          s.Doc,
			Path:         s.Path,
			StartLine:    s.StartLine,
			EndLine:      s.EndLine,
		})
	}

	// [Divine Correlation] Link AST symbols with Memory Insights
	for i := range symbols {
		for j := range iRes.insights {
			symName := symbols[i].Name
			insight := &iRes.insights[j]
			
			pattern := `(?i)\b` + regexp.QuoteMeta(symName) + `\b`
			matchedTitle, _ := regexp.MatchString(pattern, insight.Title)
			matchedWhy, _ := regexp.MatchString(pattern, insight.Why)
			matchedLearned, _ := regexp.MatchString(pattern, insight.Learned)

			// Check if symbol name appears in title, why, or learned sections
			if matchedTitle || matchedWhy || matchedLearned {
				
				symbols[i].LinkedInsights = append(symbols[i].LinkedInsights, insight.ID)
				insight.LinkedSymbols = append(insight.LinkedSymbols, symName)
			}
		}
	}

	return &types.HybridSearchResult{
		Symbols:  symbols,
		Insights: iRes.insights,
	}, nil
}
