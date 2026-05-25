package store

import (
	"context"
	"iter"

	"github.com/Rogercode97/scouter/internal/types"
)

type SymbolRegistry interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	SaveFileIndexBatch(ctx context.Context, items []BatchItem) error
	GetDirectoryHash(ctx context.Context, path string) (string, int64, error)
	SaveDirectoryHash(ctx context.Context, path string, hash string, mtime int64) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string, limit, offset int) ([]Symbol, error)
	GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]Symbol, error)
	GetSymbolsByStructuralHash(ctx context.Context, hash string) ([]Symbol, error)
	SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error]
	GetSymbolsByRange(ctx context.Context, path string, startLine, endLine int) ([]Symbol, error)
	GetSymbolsByPathPrefix(ctx context.Context, pathPrefix string) ([]Symbol, error)
	GetSymbolsByType(ctx context.Context, symType string) ([]Symbol, error)
	GetInterfaces(ctx context.Context) ([]Symbol, error)
	GetAllSymbols(ctx context.Context) iter.Seq2[Symbol, error]
	UpdateSymbolCentrality(ctx context.Context, name, path string, centrality int) error
	UpdateSymbolChurn(ctx context.Context, path string, score float64) error
	UpdateSymbolPageRank(ctx context.Context, name, path string, score float64) error
	GetStats(ctx context.Context) (int, int, error)
	GetAllFilePaths(ctx context.Context) ([]string, error)
	DeleteFileIndex(ctx context.Context, path string) error
	SaveDependency(ctx context.Context, dep *types.Dependency) error
	GetDependencies(ctx context.Context) ([]types.Dependency, error)
	ClearDependencies(ctx context.Context) error
	GetUnusedSymbols(ctx context.Context, includeExported bool) ([]Symbol, error)
}

type StructuralGraph interface {
	GetAllCalls(ctx context.Context) iter.Seq2[Call, error]
	SaveCall(ctx context.Context, call Call) error
	GetCallers(ctx context.Context, calleeName string, limit, offset int) ([]Call, error)
	GetCallees(ctx context.Context, callerName string) ([]Call, error)
	GetCallersRecursive(ctx context.Context, name, path string, maxDepth int) ([]Call, error)
	GetAffectedTestsRecursive(ctx context.Context, name, path string) ([]Symbol, error)
	ClearCalls(ctx context.Context, path string) error
}

type DiagnosticStore interface {
	GetAllFailedTests(ctx context.Context) iter.Seq2[types.TestResult, error]
	SaveTestResult(ctx context.Context, res *types.TestResult) error
	GetHealthReport(ctx context.Context, symbol string, failuresOnly bool) iter.Seq2[types.TestResult, error]
	ClearTestResults(ctx context.Context) error
	SaveViolation(ctx context.Context, v *types.ASTRuleMatch) error
	GetViolationsByFile(ctx context.Context, path string) ([]Violation, error)
}

type SyncEngine interface {
	ExportDelta(ctx context.Context, syncDir string) error
	ImportDelta(ctx context.Context, syncDir string) error
}

type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(context.Context, Store) error) error
	Close() error
}

type Store interface {
	SymbolRegistry
	StructuralGraph
	DiagnosticStore
	SyncEngine
	TransactionManager
}
