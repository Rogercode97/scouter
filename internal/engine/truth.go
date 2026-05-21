package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/Rogercode97/scouter/internal/domain/memory"
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
	store      store.Repository
	memory     memory.MemoryProvider
	analyzer   *AnalysisEngine
	lspMgr     *lsp.Manager
	impact     *ImpactEngine
	search     *SearchEngine
	compact    *CompactionEngine
	healer     *HealerEngine
	diagnostic *DiagnosticEngine
	ripple     *RippleEngine
	sdd        *SDDEngine
	ledger     *Ledger
	astRules   *ASTRuleEngine
	messenger  Messenger
	logger     *slog.Logger
}

// NewTruthEngine initializes a new TruthEngine with its dependencies.
func NewTruthEngine(
	store store.Repository,
	memory memory.MemoryProvider,
	analyzer *AnalysisEngine,
	lspMgr *lsp.Manager,
	impact *ImpactEngine,
	search *SearchEngine,
	compact *CompactionEngine,
	healer *HealerEngine,
	diagnostic *DiagnosticEngine,
	ripple *RippleEngine,
	sdd *SDDEngine,
	ledger *Ledger,
	astRules *ASTRuleEngine,
	messenger Messenger,
) *TruthEngine {
	return &TruthEngine{
		store:      store,
		memory:     memory,
		analyzer:   analyzer,
		lspMgr:     lspMgr,
		impact:     impact,
		search:     search,
		compact:    compact,
		healer:     healer,
		diagnostic: diagnostic,
		ripple:     ripple,
		sdd:        sdd,
		ledger:     ledger,
		astRules:   astRules,
		messenger:  messenger,
		logger:     slog.Default(),
	}
}

// Index parses, hashes and persists a file or directory to the store.
func (e *TruthEngine) Index(ctx context.Context, path string) error {
	if e.store == nil {
		return fmt.Errorf("store not initialized")
	}

	validatedPath, err := utils.ValidatePath(path)
	if err != nil {
		return err
	}

	fi, err := os.Stat(validatedPath)
	if err != nil {
		return fmt.Errorf("error stating path: %w", err)
	}

	if fi.IsDir() {
		return e.indexDirectory(ctx, validatedPath)
	}

	return e.indexFile(ctx, validatedPath)
}

func (e *TruthEngine) indexDirectory(ctx context.Context, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			// Skip blacklisted directories (security check)
			for _, blocked := range []string{".git", ".scouter", "node_modules", "vendor"} {
				if info.Name() == blocked {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Only index supported extensions
		ext := filepath.Ext(path)
		supported := []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py"}
		isSupported := false
		for _, s := range supported {
			if ext == s {
				isSupported = true
				break
			}
		}

		if !isSupported {
			return nil
		}

		if err := e.indexFile(ctx, path); err != nil {
			// Log error and continue with other files
			e.logger.Error("failed to index file", "path", path, "error", err)
		}

		return nil
	})
}

func (e *TruthEngine) indexFile(ctx context.Context, path string) error {
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
				Name:         ptr.Name,
				Type:         ptr.Type,
				PackagePath:  ptr.PackagePath,
				ReceiverType: ptr.ReceiverType,
				Signature:    ptr.Signature,
				Doc:          ptr.Doc,
				Path:         path,
				StartByte:    ptr.Range.Start,
				EndByte:      ptr.Range.End,
				StartLine:    ptr.StartLine,
				StartCol:     ptr.StartCol,
				EndLine:      ptr.EndLine,
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
		return fmt.Errorf("failed to save index for %s: %w", path, err)
	}

	// Architectural Audit (Wave 13.5)
	if e.astRules != nil {
		violations, err := e.astRules.Audit(ctx, path)
		if err == nil && len(violations) > 0 {
			_ = e.store.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
				for _, v := range violations {
					_ = tx.SaveViolation(ctx, &v)
				}
				return nil
			})
		}
	}

	// Post-indexing resolution (only if analyzer is present)
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
	if e.diagnostic == nil {
		return nil, fmt.Errorf("diagnostic engine not initialized")
	}

	risk, err := e.diagnostic.AssessRisk(ctx, symbol, path)
	if err != nil {
		return nil, err
	}

	// Oracle Logic
	if risk.RiskScore >= 0.8 && messenger != nil {
		prompt := fmt.Sprintf("The function '%s' in '%s' has a CRITICAL Risk Score of %.4f. Based on its centrality and blast radius, please provide a brief architectural refactoring proposal to reduce its impact.", symbol, path, risk.RiskScore)
		_, _ = messenger.Ask(ctx, "You are an expert software architect.", prompt)
	}

	// Maintain backward compatibility with the return type for now
	return e.impact.Analyze(ctx, symbol, path, 5)
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

// FindLogicalTwins finds symbols with the same structural hash as the target.
func (e *TruthEngine) FindLogicalTwins(ctx context.Context, symbolName, path string) ([]types.Symbol, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	cleanPath, err := utils.ValidatePath(path)
	if err != nil {
		return nil, err
	}

	// 1. Find the target symbol to get its hash
	symbols, err := e.store.GetSymbolsByNameInFile(ctx, symbolName, cleanPath)
	if err != nil || len(symbols) == 0 {
		// Try indexing if not found
		if err := e.Index(ctx, cleanPath); err == nil {
			symbols, _ = e.store.GetSymbolsByNameInFile(ctx, symbolName, cleanPath)
		}
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("symbol '%s' not found in %s", symbolName, path)
	}

	target := symbols[0]
	if target.StructuralHash == "" {
		return nil, fmt.Errorf("symbol '%s' has no structural hash", symbolName)
	}

	// 2. Search for twins
	twins, err := e.store.GetSymbolsByStructuralHash(ctx, target.StructuralHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find twins: %w", err)
	}

	// 3. Filter and Map
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
		})
	}

	return results, nil
}

// Fix attempts to autonomously fix a test failure via the DiagnosticEngine.
func (e *TruthEngine) Fix(ctx context.Context, errorLog string, messenger Messenger) (string, error) {
	if e.diagnostic == nil {
		return "", fmt.Errorf("diagnostic engine not initialized")
	}

	report, err := e.diagnostic.Diagnose(ctx, errorLog)
	if err != nil {
		return "", err
	}

	res, err := e.diagnostic.Heal(ctx, report, messenger)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Status: %s\nFile: %s\nFixed Code:\n%s\nTest Output:\n%s", res.Status, res.Metadata["failingFile"], res.FixedCode, res.TestOutput), nil
}

// Propagate applies an architectural refactor across the codebase.
func (e *TruthEngine) Propagate(ctx context.Context, symbol, transformation string, messenger Messenger) (string, error) {
	if messenger != nil {
		e.ripple.Transformer = NewMCPTransformer(e.store, func(ctx context.Context, file, sym, prompt string) (string, error) {
			return messenger.Ask(ctx, "You are a surgical refactoring agent.", prompt)
		})
	}
	ledger, err := e.ripple.Propagate(ctx, symbol, transformation, 5)
	if err != nil {
		if ledger != nil && len(ledger.StagedFiles()) > 0 {
			// Validation failed but changes were staged
			return fmt.Sprintf("❌ Validation failed: %v. Staged files: %v", err, ledger.StagedFiles()), err
		}
		return "", err
	}

	return fmt.Sprintf("✅ Transformation staged in Ledger for %d files: %v. Use 'scouter_commit' to apply or 'scouter_diff' to review.", len(ledger.AffectedFiles()), ledger.AffectedFiles()), nil
}

// CommitLedger applies all staged changes to disk.
func (e *TruthEngine) CommitLedger(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}
	
	files := e.ledger.StagedFiles()
	if len(files) == 0 {
		return "No changes staged in Ledger.", nil
	}

	if err := e.ledger.CommitStaged(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Committed changes to %d files: %v", len(files), files), nil
}

// RollbackLedger clears all staged changes.
func (e *TruthEngine) RollbackLedger(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	if err := e.ledger.Rollback(ctx); err != nil {
		return "", err
	}

	return "✅ Ledger rolled back. All staged changes cleared.", nil
}

// GetLedgerSummary returns a summary of the current staged changes.
func (e *TruthEngine) GetLedgerSummary(ctx context.Context) string {
	if e.ledger == nil {
		return "Ledger not initialized."
	}
	return e.ledger.Summary()
}

// GetLedgerDiff returns the diff of all staged changes.
func (e *TruthEngine) GetLedgerDiff(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	patches := e.ledger.GetStaged()
	if len(patches) == 0 {
		return "No changes staged in Ledger.", nil
	}

	var sb strings.Builder
	for _, p := range patches {
		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(p.Original),
			B:        difflib.SplitLines(p.NewContent),
			FromFile: p.FilePath + " (Original)",
			ToFile:   p.FilePath + " (Staged)",
			Context:  3,
		})
		if err != nil {
			// Fallback if diff generation fails
			sb.WriteString(fmt.Sprintf("--- %s (Original)\n+++ %s (Staged)\n", p.FilePath, p.FilePath))
			if p.Diff != "" {
				sb.WriteString(p.Diff)
			} else {
				sb.WriteString(" (Diff not available, full content staged)\n")
			}
			sb.WriteString("\n")
			continue
		}
		
		if diff == "" {
			sb.WriteString(fmt.Sprintf("--- %s (Original)\n+++ %s (Staged)\n (No changes)\n\n", p.FilePath, p.FilePath))
		} else {
			sb.WriteString(diff)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// StageMutation stages a single file change in the Ledger.
func (e *TruthEngine) StageMutation(ctx context.Context, filePath, newContent string) error {
	if e.ledger == nil {
		return fmt.Errorf("ledger not initialized")
	}

	cleanPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return err
	}

	original, _ := os.ReadFile(cleanPath)
	
	patch := Patch{
		FilePath:   cleanPath,
		Original:   string(original),
		NewContent: newContent,
	}

	return e.ledger.Stage(cleanPath, patch)
}

// GetSDDRoadmap retrieves the project roadmap from the SDD engine.
func (e *TruthEngine) GetSDDRoadmap(ctx context.Context) (*SDDRoadmap, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.ParseRoadmap(ctx)
}

// GetSDDTasks retrieves the current tasks from the SDD engine.
func (e *TruthEngine) GetSDDTasks(ctx context.Context) ([]SDDTask, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.ParseTasks(ctx)
}

// SearchSDDSpecs searches for specifications using the SDD engine.
func (e *TruthEngine) SearchSDDSpecs(ctx context.Context, query string, limit, offset int) ([]SpecResult, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.SearchSpecs(ctx, query, limit, offset)
}

