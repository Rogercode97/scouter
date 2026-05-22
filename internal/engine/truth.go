package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)




var (
	BlockedDirs = map[string]bool{
		".git":         true,
		".scouter":     true,
		"node_modules": true,
		"vendor":       true,
	}
	SupportedExts = map[string]bool{
		".go":  true,
		".ts":  true,
		".tsx": true,
		".js":  true,
		".jsx": true,
		".py":  true,
	}
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

func (e *TruthEngine) MemoryProvider() memory.MemoryProvider {
	return e.memory
}

// Index parses, hashes and persists a file or directory to the store.

type indexCollector struct {
	items     []store.BatchItem
	ch        chan store.BatchItem
	ctx       context.Context
	store     store.Repository
	done      chan struct{}
	err       error
	batchSize int
}

func calculateBatchSize() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Alloc > 1024*1024*1024 { // > 1GB
		return 100
	}
	if m.Alloc > 512*1024*1024 { // > 512MB
		return 250
	}
	return 500
}

func newIndexCollector(ctx context.Context, s store.Repository) *indexCollector {
	bs := calculateBatchSize()
	c := &indexCollector{
		items:     make([]store.BatchItem, 0, bs),
		ch:        make(chan store.BatchItem, 100),
		ctx:       ctx,
		store:     s,
		done:      make(chan struct{}),
		batchSize: bs,
	}
	go c.run()
	return c
}

func (c *indexCollector) run() {
	defer close(c.done)
	for item := range c.ch {
		c.items = append(c.items, item)
		if len(c.items) >= c.batchSize {
			if err := c.flush(); err != nil && c.err == nil {
				c.err = err
			}
		}
	}
	if len(c.items) > 0 {
		if err := c.flush(); err != nil && c.err == nil {
			c.err = err
		}
	}
}

func (c *indexCollector) flush() error {
	if len(c.items) == 0 {
		return nil
	}
	err := c.store.SaveFileIndexBatch(c.ctx, c.items)
	c.items = c.items[:0]
	return err
}

func (c *indexCollector) Wait() error {
	<-c.done
	return c.err
}

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

	collector := newIndexCollector(ctx, e.store)

	// Limitamos a 4 workers para evitar que el OS (ej. MIUI) mate el proceso por exceso de hilos
	maxWorkers := 4
	if runtime.NumCPU() < 4 {
		maxWorkers = runtime.NumCPU()
	}
	workerSem := make(chan struct{}, maxWorkers)

	var indexErr error
	if fi.IsDir() {
		_, indexErr = e.indexDirectory(ctx, validatedPath, workerSem, collector)
	} else {
		_, indexErr = e.indexFile(ctx, validatedPath, workerSem, collector)
	}

	// CERRAMOS EL CANAL AQUÍ SOLO DESPUÉS DE QUE indexDirectory/indexFile (y todos sus sub-workers) HAYAN TERMINADO
	close(collector.ch)
	collErr := collector.Wait()

	if indexErr != nil {
		return indexErr
	}
	if collErr != nil {
		return fmt.Errorf("collector failed: %w", collErr)
	}
	
	if e.analyzer != nil {
		if aErr := e.analyzer.ResolveInterfaces(ctx); aErr != nil {
			e.logger.Error("failed to resolve interfaces", "error", aErr)
		}
		if aErr := e.analyzer.ResolveCentrality(ctx); aErr != nil {
			e.logger.Error("failed to resolve centrality", "error", aErr)
		}
	}

	return err
}


func (e *TruthEngine) indexDirectory(ctx context.Context, dir string, workerSem chan struct{}, collector *indexCollector) (string, error) {
	storedHash, _, err := e.store.GetDirectoryHash(ctx, dir)
	if err == nil && storedHash != "" {
		// Cache hit logic
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("error reading directory: %w", err)
	}

	var mu sync.Mutex
	var hashes []string
	var g errgroup.Group

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		path := filepath.Join(dir, entry.Name())
		entry := entry

		if entry.IsDir() {
			if BlockedDirs[entry.Name()] {
				continue
			}
			g.Go(func() error {
				childHash, err := e.indexDirectory(ctx, path, workerSem, collector)
				if err != nil {
					e.logger.Error("failed to index directory", "path", path, "error", err)
					return nil
				}
				if childHash != "" {
					mu.Lock()
					hashes = append(hashes, childHash)
					mu.Unlock()
				}
				return nil
			})
		} else {
			ext := filepath.Ext(path)
			if !SupportedExts[ext] {
				continue
			}
			g.Go(func() error {
				childHash, err := e.indexFile(ctx, path, workerSem, collector)
				if err != nil {
					e.logger.Error("failed to index file", "path", path, "error", err)
					return nil
				}
				if childHash != "" {
					mu.Lock()
					hashes = append(hashes, childHash)
					mu.Unlock()
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return "", err
	}

	sort.Strings(hashes)
	dirHash := utils.StringHash(strings.Join(hashes, ""))

	if storedHash == dirHash {
		return storedHash, nil
	}

	err = e.store.SaveDirectoryHash(ctx, dir, dirHash, 0)
	if err != nil {
		return "", fmt.Errorf("failed to save directory hash: %w", err)
	}

	return dirHash, nil
}

func (e *TruthEngine) indexFile(ctx context.Context, path string, workerSem chan struct{}, collector *indexCollector) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("error stating file: %w", err)
	}
	mtime := fi.ModTime().UnixNano()

	existingIdx, err := e.store.GetFileIndex(ctx, path)
	if err == nil && existingIdx != nil {
		if existingIdx.Mtime == mtime {
			return existingIdx.Hash, nil // Truly unchanged, skip everything
		}
	}

	hash, err := utils.CalculateHash(path)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	if existingIdx != nil && existingIdx.Hash == hash {
		existingIdx.Mtime = mtime
		err = e.store.SaveFileIndex(ctx, existingIdx)
		if err != nil {
			return "", fmt.Errorf("failed to update file index: %w", err)
		}
		return hash, nil
	}

	// Wait for worker slot
	workerSem <- struct{}{}
	defer func() { <-workerSem }()

	e.logger.Info("indexing file", "path", path)

	itPointers, itCalls, err := StreamSymbols(ctx, path)
	if err != nil {
		return "", fmt.Errorf("parsing failed for %s: %w", path, err)
	}

	batchItem := store.BatchItem{
		Index: &store.FileIndex{
			Path:    path,
			Mtime:   mtime,
			Hash:    hash,
			ASTJSON: "{}",
			Project: utils.GetRepoName(ctx),
		},
		Symbols:    []store.Symbol{},
		Calls:      []store.Call{},
		Violations: []store.Violation{},
	}

	for ptr := range itPointers {
		batchItem.Symbols = append(batchItem.Symbols, store.Symbol{
			Name:           ptr.Name,
			Type:           ptr.Type,
			PackagePath:    ptr.PackagePath,
			ReceiverType:   ptr.ReceiverType,
			Signature:      ptr.Signature,
			Doc:            ptr.Doc,
			Path:           path,
			StartByte:      ptr.Range.Start,
			EndByte:        ptr.Range.End,
			StartLine:      ptr.StartLine,
			StartCol:       ptr.StartCol,
			EndLine:        ptr.EndLine,
			StructuralHash: ptr.StructuralHash,
		})
	}

	for c := range itCalls {
		batchItem.Calls = append(batchItem.Calls, store.Call{
			CallerName: c.CallerName,
			CalleeName: c.CalleeName,
			CalleePath: c.CalleePath,
			LinkType:   c.LinkType,
			Path:       path,
			Line:       c.Line,
		})
	}

	if e.astRules != nil {
		violations, err := e.astRules.Audit(ctx, path)
		if err == nil && len(violations) > 0 {
			for _, v := range violations {
				batchItem.Violations = append(batchItem.Violations, store.Violation{
					RuleID:    v.RuleID,
					FilePath:  v.File,
					Message:   v.Message,
					Severity:  v.Severity,
					StartLine: v.Range.Start.Line,
					StartCol:  v.Range.Start.Column,
					Text:      v.Text,
				})
			}
		}
	}

	// Envío seguro al colector con soporte para cancelación
	select {
	case collector.ch <- batchItem:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	return hash, nil
}

func (e *TruthEngine) GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error) {
	if e.analyzer == nil {
		return nil, fmt.Errorf("analysis engine not initialized")
	}
	return e.analyzer.GetCriticalSymbols(ctx, limit)
}

func (e *TruthEngine) AnalyzeImpact(ctx context.Context, symbol, path string, verbose bool, messenger Messenger) (*types.ImpactResult, error) {
	if e.diagnostic == nil {
		return nil, fmt.Errorf("diagnostic engine not initialized")
	}

	risk, err := e.diagnostic.AssessRisk(ctx, symbol, path)
	if err != nil {
		return nil, err
	}

	if risk.RiskScore >= 0.8 && messenger != nil {
		prompt := fmt.Sprintf("The function '%s' in '%s' has a CRITICAL Risk Score of %.4f. Based on its centrality and blast radius, please provide a brief architectural refactoring proposal to reduce its impact.", symbol, path, risk.RiskScore)
		_, err := messenger.Ask(ctx, "You are an expert software architect.", prompt)
		if err != nil {
			e.logger.Error("oracle ask failed", "error", err)
		}
	}

	return e.impact.Analyze(ctx, symbol, path, 5)
}

func (e *TruthEngine) PredictTests(ctx context.Context, diff string) ([]types.TestTarget, error) {
	if e.impact == nil {
		return nil, fmt.Errorf("impact engine not initialized")
	}
	return e.impact.PredictTests(ctx, diff)
}

func (e *TruthEngine) HybridSearch(ctx context.Context, query string, limit, offset int) (*types.HybridSearchResult, error) {
	return e.search.HybridSearch(ctx, query, limit, offset)
}

func (e *TruthEngine) CompactSession(ctx context.Context, log string) (*types.CompactionResult, error) {
	return e.compact.CompactSession(ctx, log)
}

func (e *TruthEngine) IdentifyCriticalContext(ctx context.Context, diff string) ([]types.ImpactEntity, error) {
	if e.impact == nil {
		return nil, fmt.Errorf("impact engine not initialized")
	}
	return e.impact.IdentifyCriticalContext(ctx, diff)
}

func (e *TruthEngine) FindLogicalTwins(ctx context.Context, symbolName, path string) ([]types.Symbol, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	cleanPath, err := utils.ValidatePath(path)
	if err != nil {
		return nil, err
	}

	symbols, err := e.store.GetSymbolsByNameInFile(ctx, symbolName, cleanPath)
	if err != nil || len(symbols) == 0 {
		if err := e.Index(ctx, cleanPath); err == nil {
			var errIdx error
			symbols, errIdx = e.store.GetSymbolsByNameInFile(ctx, symbolName, cleanPath)
			if errIdx != nil {
				return nil, fmt.Errorf("failed to get symbols after indexing: %w", errIdx)
			}
		}
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("symbol '%s' not found in %s", symbolName, path)
	}

	target := symbols[0]
	if target.StructuralHash == "" {
		return nil, fmt.Errorf("symbol '%s' has no structural hash", symbolName)
	}

	twins, err := e.store.GetSymbolsByStructuralHash(ctx, target.StructuralHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find twins: %w", err)
	}

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

func (e *TruthEngine) Propagate(ctx context.Context, symbol, transformation string, messenger Messenger) (string, error) {
	if messenger != nil {
		e.ripple.Transformer = NewMCPTransformer(e.store, func(ctx context.Context, file, sym, prompt string) (string, error) {
			return messenger.Ask(ctx, "You are a surgical refactoring agent.", prompt)
		})
	}
	ledger, err := e.ripple.Propagate(ctx, symbol, transformation, 5)
	if err != nil {
		if ledger != nil && len(ledger.StagedFiles()) > 0 {
			return fmt.Sprintf("❌ Validation failed: %v. Staged files: %v", err, ledger.StagedFiles()), err
		}
		return "", err
	}

	return fmt.Sprintf("✅ Transformation staged in Ledger for %d files: %v. Use 'scouter_commit' to apply or 'scouter_diff' to review.", len(ledger.AffectedFiles()), ledger.AffectedFiles()), nil
}

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

func (e *TruthEngine) RollbackLedger(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	if err := e.ledger.Rollback(ctx); err != nil {
		return "", err
	}

	return "✅ Ledger rolled back. All staged changes cleared.", nil
}

func (e *TruthEngine) GetLedgerSummary(ctx context.Context) string {
	if e.ledger == nil {
		return "Ledger not initialized."
	}
	return e.ledger.Summary()
}

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

func (e *TruthEngine) GetSDDRoadmap(ctx context.Context) (*SDDRoadmap, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.ParseRoadmap(ctx)
}

func (e *TruthEngine) GetSDDTasks(ctx context.Context) ([]SDDTask, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.ParseTasks(ctx)
}

func (e *TruthEngine) SearchSDDSpecs(ctx context.Context, query string, limit, offset int) ([]SpecResult, error) {
	if e.sdd == nil {
		return nil, fmt.Errorf("SDD engine not initialized")
	}
	return e.sdd.SearchSpecs(ctx, query, limit, offset)
}
