package engine

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
)

// HybridSearcher wraps Bleve to provide BM25 + Vector ranking (RRF) capabilities.
type HybridSearcher struct {
	index bleve.Index
}

// NewHybridSearcher initializes a new in-memory Bleve index for fast, 
// session-based hybrid searching.
func NewHybridSearcher() (*HybridSearcher, error) {
	indexMapping := bleve.NewIndexMapping()
	
	// Create an in-memory index for maximum speed during the agent session
	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create bleve index: %w", err)
	}
	
	return &HybridSearcher{index: index}, nil
}

// IndexSymbol adds a structural symbol to the search engine.
func (s *HybridSearcher) IndexSymbol(id string, data map[string]interface{}) error {
	return s.index.Index(id, data)
}

// SearchBM25 executes a fast text-relevance search.
func (s *HybridSearcher) SearchBM25(queryStr string, limit int) (*bleve.SearchResult, error) {
	query := bleve.NewMatchQuery(queryStr)
	req := bleve.NewSearchRequest(query)
	req.Size = limit
	// Future: req.Score = bleve.ScoreRRF when combining with KNN
	return s.index.Search(req)
}
