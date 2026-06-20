package engine

import (
	"context"
	"fmt"
	"log/slog"

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
	store      store.Store
	memory     memory.MemoryProvider
	analyzer   *AnalysisEngine
	lspMgr     *lsp.Manager
	impact     *ImpactEngine
	search     *SearchEngine
	compact    *CompactionEngine
	healer     *HealerEngine
	diagnostic *DiagnosticEngine
	semantic   *SemanticEngine
	ripple     *RippleEngine
	ledger     *Ledger
	astRules   *ASTRuleEngine
	messenger  Messenger
	logger     *slog.Logger
}

// TruthOption defines a functional option for configuring a TruthEngine.
type TruthOption func(*TruthEngine)

func WithMemory(m memory.MemoryProvider) TruthOption {
	return func(te *TruthEngine) { te.memory = m }
}

func WithAnalyzer(a *AnalysisEngine) TruthOption {
	return func(te *TruthEngine) { te.analyzer = a }
}

func WithLSP(l *lsp.Manager) TruthOption {
	return func(te *TruthEngine) { te.lspMgr = l }
}

func WithImpact(i *ImpactEngine) TruthOption {
	return func(te *TruthEngine) { te.impact = i }
}

func WithSearch(s *SearchEngine) TruthOption {
	return func(te *TruthEngine) { te.search = s }
}

func WithCompact(c *CompactionEngine) TruthOption {
	return func(te *TruthEngine) { te.compact = c }
}

func WithHealer(h *HealerEngine) TruthOption {
	return func(te *TruthEngine) { te.healer = h }
}

func WithDiagnostic(d *DiagnosticEngine) TruthOption {
	return func(te *TruthEngine) { te.diagnostic = d }
}

func WithSemantic(s *SemanticEngine) TruthOption {
	return func(te *TruthEngine) { te.semantic = s }
}

func WithRipple(r *RippleEngine) TruthOption {
	return func(te *TruthEngine) { te.ripple = r }
}

func WithLedger(l *Ledger) TruthOption {
	return func(te *TruthEngine) { te.ledger = l }
}

func WithASTRules(a *ASTRuleEngine) TruthOption {
	return func(te *TruthEngine) { te.astRules = a }
}

func WithMessenger(m Messenger) TruthOption {
	return func(te *TruthEngine) { te.messenger = m }
}

// NewTruthEngine initializes a new TruthEngine with its dependencies.
func NewTruthEngine(store store.Store, opts ...TruthOption) *TruthEngine {
	te := &TruthEngine{
		store:  store,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(te)
	}
	return te
}

func (e *TruthEngine) MemoryProvider() memory.MemoryProvider {
	return e.memory
}

// Index parses, hashes and persists a file or directory to the store.

func (e *TruthEngine) Index(ctx context.Context, path string) error {
	pipeline := NewIndexerPipeline(IndexerConfig{
		Store:    e.store,
		Semantic: e.semantic,
		Analyzer: e.analyzer,
		Search:   e.search,
		ASTRules: e.astRules,
		Logger:   e.logger,
	})
	return pipeline.Run(ctx, path)
}
func (e *TruthEngine) GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error) {
	if e.analyzer == nil {
		return nil, fmt.Errorf("analysis engine not initialized")
	}
	return e.analyzer.GetCriticalSymbols(ctx, limit)
}

func (e *TruthEngine) AuditArchitecture(ctx context.Context, targetPath string) ([]types.ASTRuleMatch, error) {
	if e.astRules == nil {
		return nil, fmt.Errorf("ast rule engine not initialized")
	}
	return e.astRules.Audit(ctx, targetPath)
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

func (e *TruthEngine) Compact() *CompactionEngine {
	return e.compact
}

func (e *TruthEngine) Healer() *HealerEngine {
	return e.healer
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

func (e *TruthEngine) GetNeighborhood(ctx context.Context, filePath string) (string, error) {
	if e.analyzer == nil {
		return "", fmt.Errorf("analysis engine not initialized")
	}
	return e.analyzer.GetNeighborhood(ctx, filePath)
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
		return nil, fmt.Errorf("symbol '%s' not found in %s (file not indexed or missing)", symbolName, path)
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


func (e *TruthEngine) SemanticSearch(ctx context.Context, query string, limit int) ([]types.Symbol, error) {
	if e.semantic == nil {
		return nil, fmt.Errorf("semantic engine not initialized")
	}
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	embedding, err := e.semantic.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	storeSymbols, err := e.store.SearchSemantic(ctx, embedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute semantic search: %w", err)
	}

	var results []types.Symbol
	for _, s := range storeSymbols {
		results = append(results, types.Symbol{
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
	return results, nil
}

func (e *TruthEngine) DiagnoseHUD(ctx context.Context, errorLog string) (*DiagnosticHUD, error) {
	if e.diagnostic == nil {

		return nil, fmt.Errorf("diagnostic engine not initialized")

	}
	return e.healer.DiagnoseHUD(ctx, errorLog)
}
