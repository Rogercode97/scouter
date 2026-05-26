package engine

import (
	"context"

	"github.com/Rogercode97/scouter/internal/store"
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
// This decouples RippleEngine and others from depending on concrete *ImpactEngine pointers.
type ImpactAnalyzer interface {
	GetDeterministicCallers(ctx context.Context, symbolName string) ([]store.Call, error)
}
