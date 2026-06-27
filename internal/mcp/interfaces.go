package mcp

import (
	"context"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// Indexer defines the interface for the indexer pipeline.
type Indexer interface {
	Index(ctx context.Context, path string) error
}

// Searcher defines the interface for the search engine.
type Searcher interface {
	HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error)
	FindLogicalTwins(ctx context.Context, symbolName, path string) ([]types.Symbol, error)
}

// Analyzer defines the interface for the analysis engine.
type Analyzer interface {
	GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error)
	GetNeighborhood(ctx context.Context, filePath string) (string, error)
}

// ImpactAssessor defines the interface for the impact engine.
type ImpactAssessor interface {
	Analyze(ctx context.Context, symbol string, path string, maxDepth int, opts ...engine.AnalyzeOption) (*types.ImpactResult, error)
	PredictTests(ctx context.Context, diff string) ([]types.TestTarget, error)
}

// Diagnoser defines the interface for the diagnostic engine.
type Diagnoser interface {
	AssessRisk(ctx context.Context, symbol, path string) (*engine.RiskAssessment, error)
	Diagnose(ctx context.Context, errorLog string) (*engine.DiagnosticReport, error)
	DiagnoseHUD(ctx context.Context, errorLog string) (*engine.DiagnosticHUD, error)
}

// ASTRulesEngine defines the interface for the AST rules engine.
type ASTRulesEngine interface {
	Audit(ctx context.Context, targetPath string) ([]types.ASTRuleMatch, error)
}

// EvolutionaryEngine defines the interface for the evolution engine.
type EvolutionaryEngine interface {
	Propagate(ctx context.Context, symbol, transformation string, messenger engine.Messenger) (string, error)
	CommitLedger(ctx context.Context) (string, error)
	RollbackLedger(ctx context.Context) (string, error)
	GetLedgerDiff(ctx context.Context) (string, error)
	GetLedgerSummary(ctx context.Context) string
	ProposeEvolution(ctx context.Context, proposal string, force bool, messenger engine.Messenger) (string, error)
}

// HealerEngine defines the interface for the healer engine.
type HealerEngine interface {
	SetFixRequestHook(hook func(ctx context.Context, prompt string) (string, error))
	Fix(ctx context.Context, errorLog string) (*types.HealResult, error)
}

// ChronosEngine defines the interface for the chronos engine.
type ChronosEngine interface {
	TakeSnapshot(ctx context.Context, filePath string) (*engine.ChronosSnapshot, error)
	CompareSnapshot(ctx context.Context, snapshot *engine.ChronosSnapshot, currentFilePath string) (*engine.ChronosDiff, error)
	SemanticDiff(ctx context.Context, target string) (string, error)
}
