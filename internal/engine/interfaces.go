package engine

import (
	"context"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// GraphStore defines the core read-only graph traversal capabilities.
type GraphStore interface {
	store.SymbolRegistry
	store.StructuralGraph
}

// TransactionalStore defines the complete contract required by HealerEngine.
type TransactionalStore interface {
	GraphStore
	store.DiagnosticStore
	store.TransactionManager
}

// ImpactAnalyzer abstracts the capabilities of the ImpactEngine required by other engines.
type ImpactAnalyzer interface {
	GetDeterministicCallers(ctx context.Context, symbolName string) ([]store.Call, error)
}

type IndexerService interface {
	Index(ctx context.Context, path string) error
}

type DiscoveryService interface {
	HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error)
	FindLogicalTwins(ctx context.Context, symbolName, path string) ([]types.Symbol, error)
	GetNeighborhood(ctx context.Context, filePath string) (string, error)
}

type IntelligenceService interface {
	AnalyzeImpact(ctx context.Context, symbol, path string, verbose bool, messenger Messenger) (*types.ImpactResult, error)
	GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error)
	PredictTests(ctx context.Context, diff string) ([]types.TestTarget, error)
	AuditArchitecture(ctx context.Context, targetPath string) ([]types.ASTRuleMatch, error)
}

type EvolutionService interface {
	ProposeEvolution(ctx context.Context, proposal string, force bool, messenger Messenger) (string, error)
	Propagate(ctx context.Context, symbol, transformation string, messenger Messenger) (string, error)
	CommitLedger(ctx context.Context) (string, error)
	RollbackLedger(ctx context.Context) (string, error)
	GetLedgerDiff(ctx context.Context) (string, error)
	GetLedgerSummary(ctx context.Context) string
	StageMutation(ctx context.Context, filePath, newContent string) error
}

type HealerService interface {
	Fix(ctx context.Context, errorLog string, messenger Messenger) (string, error)
	DiagnoseHUD(ctx context.Context, errorLog string) (*DiagnosticHUD, error)
}
