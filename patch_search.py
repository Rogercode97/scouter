import os
import re

def replace_in_file(path, old, new):
    if not os.path.exists(path):
        print(f"Skipping {path}, not found.")
        return
    with open(path, "r") as f:
        content = f.read()
    content = content.replace(old, new)
    with open(path, "w") as f:
        f.write(content)

# 1. Update NewSearchEngine usage in cli.go
replace_in_file("internal/cli/cli.go", "engine.NewSearchEngine(db, memoryProvider)", "engine.NewSearchEngine(db, memoryProvider, semanticEngine)")
replace_in_file("internal/cli/cli.go", "engine.NewSearchEngine(db, nil)", "engine.NewSearchEngine(db, nil, nil)")

# 2. Update healer_test.go
replace_in_file("internal/engine/healer_test.go", "NewSearchEngine(s, nil)", "NewSearchEngine(s, nil, nil)")

# 3. Update test_helper.go
replace_in_file("internal/mcp/test_helper.go", "engine.NewSearchEngine(db, memoryProvider)", "engine.NewSearchEngine(db, memoryProvider, nil)")

# 4. Update search.go
with open("internal/engine/search.go", "r") as f:
    search_content = f.read()

# Update struct and constructor
search_old_struct = """type SearchEngine struct {
	store  store.SymbolRegistry
	memory memory.MemoryProvider
}

func NewSearchEngine(s store.SymbolRegistry, m memory.MemoryProvider) *SearchEngine {
	return &SearchEngine{store: s, memory: m}
}"""

search_new_struct = """type SearchEngine struct {
	store    store.SymbolRegistry
	memory   memory.MemoryProvider
	semantic *SemanticEngine
}

func NewSearchEngine(s store.SymbolRegistry, m memory.MemoryProvider, sem *SemanticEngine) *SearchEngine {
	return &SearchEngine{store: s, memory: m, semantic: sem}
}"""

search_content = search_content.replace(search_old_struct, search_new_struct)

# Update HybridSearch function
search_old_hybrid = """func (e *SearchEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {"""

# We'll replace the whole function by finding its bounds
# Find start
start_idx = search_content.find("func (e *SearchEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {")

# Find end (IndexSymbol func)
end_idx = search_content.find("func (e *SearchEngine) IndexSymbol", start_idx)

new_hybrid = """func (e *SearchEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {
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
		})
	}

	// [Divine Correlation] Link AST symbols with Memory Insights
	for i := range symbols {
		symName := symbols[i].Name
		pattern := `(?i)\\b` + regexp.QuoteMeta(symName) + `\\b`
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

"""

search_content = search_content[:start_idx] + new_hybrid + search_content[end_idx:]

if '"log/slog"' not in search_content:
    search_content = search_content.replace('"fmt"', '"fmt"\n\t"log/slog"')

with open("internal/engine/search.go", "w") as f:
    f.write(search_content)

print("Patch applied successfully.")
