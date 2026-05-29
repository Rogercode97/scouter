package engine

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// SearchEngine unifies AST structural search with Engram historical insights.
type SearchEngine struct {
	store  store.SymbolRegistry
	memory memory.MemoryProvider
	Bleve  *HybridSearcher
}

func NewSearchEngine(s store.SymbolRegistry, m memory.MemoryProvider) *SearchEngine {
	hs, _ := NewHybridSearcher()
	return &SearchEngine{store: s, memory: m, Bleve: hs}
}

// HybridSearch executes parallel lookups in the AST, Bleve, and Engram databases.
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

	type textRes struct {
		hits map[string]int
		err  error
	}

	symChan := make(chan symRes, 1)
	insChan := make(chan insRes, 1)
	txtChan := make(chan textRes, 1)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		res, err := e.store.SearchSymbols(ctx, query, "", limit*2, offset)
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

	go func() {
		defer wg.Done()
		if e.Bleve == nil {
			txtChan <- textRes{nil, nil}
			return
		}
		res, err := e.Bleve.SearchBM25(query, limit*2)
		if err != nil {
			txtChan <- textRes{nil, err}
			return
		}
		hits := make(map[string]int)
		for i, hit := range res.Hits {
			hits[hit.ID] = i + 1
		}
		txtChan <- textRes{hits, nil}
	}()

	wg.Wait()
	close(symChan)
	close(insChan)
	close(txtChan)

	sRes := <-symChan
	if sRes.err != nil {
		return nil, fmt.Errorf("AST search failed: %w", sRes.err)
	}

	iRes := <-insChan
	if iRes.err != nil {
		return nil, fmt.Errorf("Engram search failed: %w", iRes.err)
	}

	tRes := <-txtChan
	if tRes.err != nil {
		tRes.hits = nil
	}

	const K = 60.0
	type scoredSymbol struct {
		sym   store.Symbol
		score float64
	}
	var rrfResults []scoredSymbol

	for i, s := range sRes.symbols {
		id := s.Path + ":" + s.Name
		astRank := float64(i + 1)
		astScore := 1.0 / (K + astRank)

		txtScore := 0.0
		if tRes.hits != nil {
			if txtRank, ok := tRes.hits[id]; ok {
				txtScore = 1.0 / (K + float64(txtRank))
			}
		}

		rrfResults = append(rrfResults, scoredSymbol{
			sym:   s,
			score: astScore + txtScore,
		})
	}

	sort.SliceStable(rrfResults, func(i, j int) bool {
		return rrfResults[i].score > rrfResults[j].score
	})

	var symbols []types.Symbol
	end := limit
	if end > len(rrfResults) {
		end = len(rrfResults)
	}

	for i := 0; i < end; i++ {
		s := rrfResults[i].sym
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

func (e *SearchEngine) IndexSymbol(docID string, data map[string]interface{}) error {
	if e.Bleve != nil {
		return e.Bleve.IndexSymbol(docID, data)
	}
	return nil
}
