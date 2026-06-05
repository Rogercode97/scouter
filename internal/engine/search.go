package engine

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

type SearchEngine struct {
	store    store.SymbolRegistry
	memory   memory.MemoryProvider
	semantic *SemanticEngine
}

func NewSearchEngine(s store.SymbolRegistry, m memory.MemoryProvider, sem *SemanticEngine) *SearchEngine {
	return &SearchEngine{store: s, memory: m, semantic: sem}
}

// HybridSearch executes parallel lookups in the AST, Bleve, and Engram databases.
func (e *SearchEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("missing query")
	}

	type insRes struct {
		insights []types.MemoryInsight
		err      error
	}

	insChan := make(chan insRes, 1)
	go func() {
		if e.memory == nil {
			insChan <- insRes{nil, nil}
			return
		}
		res, err := e.memory.SearchInsights(ctx, query, 5)
		insChan <- insRes{res, err}
	}()

	var storeSymbols []store.Symbol
	var err error

	if e.semantic != nil {
		embedding, semErr := e.semantic.GenerateEmbedding(ctx, query)
		if semErr != nil {
			// Elegant Degradation: Warning and fallback to FTS5
			fmt.Printf("WARNING: semantic.GenerateEmbedding failed: %v. Falling back to FTS5 search.\n", semErr)
			storeSymbols, err = e.store.SearchSymbols(ctx, query, "", limit, offset)
		} else {
			storeSymbols, err = e.store.SearchHybrid(ctx, query, embedding, limit)
		}
	} else {
		storeSymbols, err = e.store.SearchSymbols(ctx, query, "", limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("store search failed: %w", err)
	}

	iRes := <-insChan
	if iRes.err != nil {
		return nil, fmt.Errorf("Engram search failed: %w", iRes.err)
	}

	var symbols []types.Symbol
	for _, s := range storeSymbols {
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

func (e *SearchEngine) IndexSymbol(docID string, data map[string]interface{}) error {
	// Replaced by SQLite Triggers in FTS5
	return nil
}
