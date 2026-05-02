package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// Messenger defines the interface for the TruthEngine to communicate with the user
// via the underlying protocol (e.g., MCP Sampling).
type Messenger interface {
	Ask(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// TruthEngine is the central orchestrator for all "truth-seeking" operations.
// It decouples the MCP handlers from the core logic and manages the coordination
// between different specialized engines.
type TruthEngine struct {
	store     store.Repository
	analyzer  *AnalysisEngine
	lspMgr    *lsp.Manager
	impact    *ImpactEngine
	search    *SearchEngine
	compact   *CompactionEngine
	healer    *HealerEngine
	ripple    *RippleEngine
	messenger Messenger
	logger    *slog.Logger
}

// NewTruthEngine initializes a new TruthEngine with its dependencies.
func NewTruthEngine(
	store store.Repository,
	analyzer *AnalysisEngine,
	lspMgr *lsp.Manager,
	impact *ImpactEngine,
	search *SearchEngine,
	compact *CompactionEngine,
	healer *HealerEngine,
	ripple *RippleEngine,
	messenger Messenger,
) *TruthEngine {
	return &TruthEngine{
		store:     store,
		analyzer:  analyzer,
		lspMgr:    lspMgr,
		impact:    impact,
		search:    search,
		compact:   compact,
		healer:    healer,
		ripple:    ripple,
		messenger: messenger,
		logger:    slog.Default(),
	}
}

// Index parses, hashes and persists a file to the store.
func (e *TruthEngine) Index(ctx context.Context, path string) error {
	if e.store == nil {
		return fmt.Errorf("store not initialized")
	}

	path, err := utils.ValidatePath(path)
	if err != nil {
		return err
	}

	itPointers, itCalls, err := StreamSymbols(ctx, path)
	if err != nil {
		return err
	}

	hash, _ := utils.CalculateHash(path)
	fi, _ := os.Stat(path)
	mtime := int64(0)
	if fi != nil {
		mtime = fi.ModTime().UnixNano()
	}

	err = e.store.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
		tx.SaveFileIndex(ctx, &store.FileIndex{
			Path:    path,
			Mtime:   mtime,
			Hash:    hash,
			ASTJSON: "{}",
			Project: utils.GetRepoName(ctx),
		})
		tx.ClearSymbols(ctx, path)
		tx.ClearCalls(ctx, path)

		for ptr := range itPointers {
			if err := tx.SaveSymbol(ctx, &store.Symbol{
				Name:      ptr.Name,
				Type:      ptr.Type,
				Signature: ptr.Signature,
				Doc:       ptr.Doc,
				Path:      path,
				StartByte: ptr.Range.Start,
				EndByte:   ptr.Range.End,
				StartLine: ptr.StartLine,
				StartCol:  ptr.StartCol,
				EndLine:   ptr.EndLine,
			}); err != nil {
				return err
			}
		}

		for c := range itCalls {
			if err := tx.SaveCall(ctx, store.Call{
				CallerName: c.CallerName,
				CalleeName: c.CalleeName,
				CalleePath: c.CalleePath,
				LinkType:   c.LinkType,
				Path:       path,
				Line:       c.Line,
			}); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	// Post-indexing resolution
	if e.analyzer != nil {
		_ = e.analyzer.ResolveInterfaces(ctx)
		_ = e.analyzer.ResolveCentrality(ctx)
	}

	return nil
}

// GetCriticalSymbols delegates to the AnalysisEngine.
func (e *TruthEngine) GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error) {
	if e.analyzer == nil {
		return nil, fmt.Errorf("analysis engine not initialized")
	}
	return e.analyzer.GetCriticalSymbols(ctx, limit)
}

// AnalyzeImpact calculates the blast radius of a symbol and potentially invokes the Oracle.
func (e *TruthEngine) AnalyzeImpact(ctx context.Context, symbol, path string, verbose bool, messenger Messenger) (*types.ImpactResult, error) {
	if e.impact == nil {
		return nil, fmt.Errorf("impact engine not initialized")
	}

	res, err := e.impact.Analyze(ctx, symbol, path, 5)
	if err != nil {
		return nil, err
	}

	if len(res.Callers) > 500 {
		res.Callers = res.Callers[:500]
	}

	// Oracle Logic
	if res.Target.RiskScore >= 0.8 && messenger != nil {
		prompt := fmt.Sprintf("The function '%s' in '%s' has a CRITICAL Risk Score of %.4f. Based on its centrality and blast radius, please provide a brief architectural refactoring proposal to reduce its impact.", symbol, path, res.Target.RiskScore)
		_, _ = messenger.Ask(ctx, "You are an expert software architect.", prompt)
	}

	return res, nil
}

// PredictTests identifies tests affected by changes described in the diff string.
func (e *TruthEngine) PredictTests(ctx context.Context, diff string) ([]types.TestTarget, error) {
	if e.impact == nil {
		return nil, fmt.Errorf("impact engine not initialized")
	}
	return e.impact.PredictTests(ctx, diff)
}

// HybridSearch performs a search across AST symbols and Engram insights.
func (e *TruthEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {
	return e.search.HybridSearch(ctx, query, limit, offset)
}

// CompactSession reduces noise in a session log.
func (e *TruthEngine) CompactSession(ctx context.Context, log string) (*types.CompactionResult, error) {
	return e.compact.CompactSession(ctx, log)
}

// IdentifyCriticalContext finds high-risk changes in a diff.
func (e *TruthEngine) IdentifyCriticalContext(ctx context.Context, diff string) ([]types.ImpactEntity, error) {
	if e.impact == nil {
		return nil, fmt.Errorf("impact engine not initialized")
	}
	return e.impact.IdentifyCriticalContext(ctx, diff)
}

// Fix attempts to autonomously fix a test failure.
func (e *TruthEngine) Fix(ctx context.Context, errorLog string, messenger Messenger) (string, error) {
	if messenger != nil {
		e.healer.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
			return messenger.Ask(ctx, "You are an autonomous Go fixing agent.", prompt)
		}
	}
	res, err := e.healer.Fix(ctx, errorLog)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Status: %s\nFile: %s\nFixed Code:\n%s\nTest Output:\n%s", res.Status, res.Metadata["failingFile"], res.FixedCode, res.TestOutput), nil
}

// Propagate applies an architectural refactor across the codebase.
func (e *TruthEngine) Propagate(ctx context.Context, symbol, transformation string, messenger Messenger) (string, error) {
	if messenger != nil {
		e.ripple.Transformer = &TruthTransformer{Messenger: messenger}
	}
	ledger, err := e.ripple.Propagate(ctx, symbol, transformation, 5)
	if err != nil {
		if ledger != nil && len(ledger.StagedFiles()) > 0 {
			// Validation failed but changes were staged (not yet committed)
			return fmt.Sprintf("❌ Validation failed: %v. Staged files: %v", err, ledger.StagedFiles()), err
		}
		return "", err
	}

	if err := ledger.CommitStaged(ctx); err != nil {
		return "", fmt.Errorf("failed to commit staged changes: %w", err)
	}

	return fmt.Sprintf("✅ Applied transformation to %d files: %v", len(ledger.AffectedFiles()), ledger.AffectedFiles()), nil
}

// TruthTransformer adapts Messenger to RippleEngine's Transformer interface.
type TruthTransformer struct {
	Messenger Messenger
}

func (t *TruthTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	content, _ := os.ReadFile(file)
	prompt := fmt.Sprintf("File: %s\nTarget Symbol: %s\nTransformation: %s\n\nSource Code:\n%s", file, symbol, transformation, string(content))
	return t.Messenger.Ask(ctx, "You are a surgical refactoring agent.", prompt)
}
