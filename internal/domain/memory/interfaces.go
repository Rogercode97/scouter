package memory

import (
	"context"

	"github.com/Rogercode97/scouter/internal/types"
)

/**
 * ⚔️ HAKAISHIN DOMAIN: Observations (WAVE 7)
 */
type Observation struct {
	Content         string   `json:"content"`
	ASTContext      string   `json:"ast_context,omitempty"`
	StructuralLinks []string `json:"structural_links,omitempty"`
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Transcript Message (WAVE 11)
 */
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Distilled Memory (WAVE 11)
 * Represents a discrete finding from a session.
 */
type DistilledMemory struct {
	Type    string `json:"type"` // architecture, bugfix, pattern
	Topic   string `json:"topic,omitempty"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Distillation Summary (WAVE 7)
 */
type Summary struct {
	ADRs     []string `json:"adrs"`
	BugFixes []string `json:"bug_fixes"`
	Patterns []string `json:"patterns"`
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Memory Provider Interface
 * Decouples the distillation logic from SQLite/Engram storage.
 */
type MemoryProvider interface {
	GetRecentObservations(ctx context.Context, project string, hours int) ([]Observation, error)
	SaveObservation(ctx context.Context, project string, memory DistilledMemory) error
	SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error)
	SaveSummary(ctx context.Context, project string, summary Summary) error
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Distiller Interface
 * Decouples the distillation logic from Gemini/Google Gen AI SDK.
 */
type Distiller interface {
	Distill(ctx context.Context, logs []Observation) (Summary, error)
	DistillTranscript(ctx context.Context, transcript []Message) ([]DistilledMemory, error)
}

/**
 * ⚔️ HAKAISHIN DOMAIN: Symbol Searcher Interface
 * Used by providers to correlate memory with structural data.
 */
type SymbolSearcher interface {
	SearchSymbols(ctx context.Context, query string, symType string, limit, offset int) ([]types.Symbol, error)
}
