package engine

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
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
			slog.Warn("semantic.GenerateEmbedding failed, falling back to FTS5 search", "error", semErr)
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
			ChurnScore:   s.ChurnScore,
		})
	}

	// [Divine Correlation] Link AST symbols with Memory Insights
	for i := range symbols {
		symName := symbols[i].Name
		pattern := `(?i)\b` + regexp.QuoteMeta(symName) + `\b`
		re := regexp.MustCompile(pattern)

		for j := range iRes.insights {
			insight := &iRes.insights[j]

			matchedTitle := re.MatchString(insight.Title)
			matchedWhy := re.MatchString(insight.Why)
			matchedLearned := re.MatchString(insight.Learned)

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

func (e *SearchEngine) FindLogicalTwins(ctx context.Context, symbolName, path string) ([]types.Symbol, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	cleanPath, err := utils.ValidatePath(path)
	if err != nil {
		return nil, err
	}

	symbols, err := e.store.GetSymbolsByNameInFile(ctx, symbolName, cleanPath)
	if err != nil || len(symbols) == 0 {
		return nil, fmt.Errorf("symbol '%s' not found in %s (file not indexed or missing)", symbolName, path)
	}

	target := symbols[0]
	if target.StructuralHash == "" {
		return nil, fmt.Errorf("symbol '%s' has no structural hash", symbolName)
	}

	twins, err := e.store.GetSymbolsByStructuralHash(ctx, target.StructuralHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find twins: %w", err)
	}

	var results []types.Symbol
	for _, twin := range twins {
		if twin.Name == target.Name && twin.Path == target.Path {
			continue
		}
		results = append(results, types.Symbol{
			Name:      twin.Name,
			Type:      twin.Type,
			Signature: twin.Signature,
			Doc:       twin.Doc,
			Path:      twin.Path,
			StartLine: twin.StartLine,
			EndLine:   twin.EndLine,
			ChurnScore: twin.ChurnScore,
		})
	}

	return results, nil
}
