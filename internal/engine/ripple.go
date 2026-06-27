package engine

import (
	"context"
	"fmt"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// PropagationTask represents a single unit of work in the ripple propagation.
type PropagationTask struct {
	SymbolName     string // Local name for matching (e.g., "Greet")
	ImpactedSymbol string // FQN of the impacted symbol (e.g., "tests.S1.Greet")
	FilePath       string
	Action         string
}

// ValidationResult carries the outcome of a validation stage.
type ValidationResult struct {
	Valid   bool
	Message string
	Details map[string]any
}

// PropagationStrategy defines the traversal logic for symbol discovery.
type PropagationStrategy interface {
	Discover(ctx context.Context, startSymbol string, depth int) iter.Seq2[PropagationTask, error]
}

// Validator defines the interface for verifying the integrity of staged changes.
type Validator interface {
	Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error)
}

// Transformer defines how to modify a file based on a symbolic change.
type Transformer interface {
	Transform(ctx context.Context, file string, symbolName string, transformation string) (string, error)
}

type RippleEngine struct {
	store        GraphStore
	Transformer  Transformer
	ImpactEngine ImpactAnalyzer
	Strategy     PropagationStrategy
	Validators   []Validator
}

func NewRippleEngine(s GraphStore, t Transformer, ie ImpactAnalyzer) *RippleEngine {
	return &RippleEngine{
		store:        s,
		Transformer:  t,
		ImpactEngine: ie,
		Strategy:     NewBFSPropagationStrategy(s, ie),
		Validators:   []Validator{},
	}
}

// BFSPropagationStrategy implements PropagationStrategy using BFS traversal.
type BFSPropagationStrategy struct {
	store        GraphStore
	ImpactEngine ImpactAnalyzer
}

// NewBFSPropagationStrategy creates a new breadth-first propagation strategy.
func NewBFSPropagationStrategy(s GraphStore, ie ImpactAnalyzer) *BFSPropagationStrategy {
	return &BFSPropagationStrategy{
		store:        s,
		ImpactEngine: ie,
	}
}

func (s *BFSPropagationStrategy) Discover(ctx context.Context, startSymbol string, maxDepth int) iter.Seq2[PropagationTask, error] {
	return func(yield func(PropagationTask, error) bool) {
		visited := make(map[string]bool)

		// Helper to get local name from FQN
		getLocalName := func(fqn string) string {
			if lastDot := strings.LastIndex(fqn, "."); lastDot != -1 {
				return fqn[lastDot+1:]
			}
			return fqn
		}

		localStartName := getLocalName(startSymbol)

		// 1. Also include the symbol definition file itself (Depth 0)
		results, _ := s.store.SearchSymbols(ctx, localStartName, "", 0, 0)
		for _, sym := range results {
			symFQ := sym.Name
			if sym.PackagePath != "" {
				symFQ = sym.PackagePath + "." + sym.Name
			}
			if symFQ == startSymbol || sym.Name == startSymbol {
				task := PropagationTask{
					SymbolName:     sym.Name,
					ImpactedSymbol: symFQ,
					FilePath:       sym.Path,
					Action:         "transform",
				}
				if !visited[task.ImpactedSymbol] {
					visited[task.ImpactedSymbol] = true
					if !yield(task, nil) {
						return
					}
				}
			}
		}

		// 2. Execute SQL Recursive Engine directly for maxDepth > 0
		if maxDepth > 0 {
			calls, err := s.store.GetRippleGraphRecursive(ctx, startSymbol, maxDepth)
			if err != nil {
				yield(PropagationTask{}, fmt.Errorf("failed to get ripple graph recursive: %w", err))
				return
			}

			for _, call := range calls {
				// Determine direction of propagation from the edge using visited state
				var impacted string
				var path string
				var symName string
				if visited[call.CalleeName] {
					impacted = call.CallerName
					symName = call.CalleeName
					path = call.Path
				} else {
					impacted = call.CalleeName
					symName = call.CallerName
					path = call.CalleePath
				}
				
				task := PropagationTask{
					SymbolName:     getLocalName(symName),
					ImpactedSymbol: impacted,
					FilePath:       path,
					Action:         "transform",
				}

				if !visited[task.ImpactedSymbol] {
					visited[task.ImpactedSymbol] = true
					if !yield(task, nil) {
						return
					}
				}
			}
		}
	}
}

// BuildValidator ensures the project still builds after changes.
type BuildValidator struct{}

func (v *BuildValidator) Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	if out, err := cmd.CombinedOutput(); err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Build failed: %v\n%s", err, string(out)),
		}, nil
	}
	return ValidationResult{Valid: true, Message: "Build successful"}, nil
}

// TestValidator ensures relevant tests still pass.
type TestValidator struct {
	SpecificTests []string
}

func (v *TestValidator) Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error) {
	args := []string{"test"}
	if len(v.SpecificTests) > 0 {
		args = append(args, v.SpecificTests...)
	} else {
		args = append(args, "./...")
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Tests failed: %v\n%s", err, string(out)),
		}, nil
	}
	return ValidationResult{Valid: true, Message: "Tests passed"}, nil
}

// CentralityValidator detects architectural regressions via centrality spikes.
type CentralityValidator struct {
	analyzer *AnalysisEngine
}

func NewCentralityValidator(a *AnalysisEngine) *CentralityValidator {
	return &CentralityValidator{analyzer: a}
}

func (v *CentralityValidator) Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error) {
	// 1. Get baseline centrality
	baseline := make(map[string]float64)
	for sym, err := range v.analyzer.store.GetAllSymbols(ctx) {
		if err != nil {
			return ValidationResult{}, err
		}
		baseline[sym.Name+":"+sym.Path] = sym.Relevance
	}

	// 2. Re-resolve centrality
	if err := v.analyzer.ResolveCentrality(ctx); err != nil {
		return ValidationResult{}, err
	}

	// 3. Compare
	var violations []string
	for sym, err := range v.analyzer.store.GetAllSymbols(ctx) {
		if err != nil {
			return ValidationResult{}, err
		}
		key := sym.Name + ":" + sym.Path
		oldVal := baseline[key]
		newVal := sym.Relevance

		if oldVal > 0 && (newVal-oldVal)/oldVal > 0.20 {
			violations = append(violations, fmt.Sprintf("centrality spike for %s: %.1f -> %.1f (+%.1f%%)", sym.Name, oldVal, newVal, (newVal-oldVal)/oldVal*100))
		}
	}

	if len(violations) > 0 {
		return ValidationResult{
			Valid:   false,
			Message: "Centrality threshold exceeded",
			Details: map[string]any{"violations": violations},
		}, nil
	}

	return ValidationResult{Valid: true, Message: "Centrality check passed"}, nil
}

// LSPValidator uses go/packages with Overlays to perform zero-disk verification
// of interface implementations across the entire workspace.
type LSPValidator struct {
	ProjectRoot string
}

func NewLSPValidator(root string) *LSPValidator {
	if root == "" {
		root, _ = filepath.Abs(".")
	}
	return &LSPValidator{ProjectRoot: root}
}

func (v *LSPValidator) Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error) {
	overlay := make(map[string][]byte)
	for path, patch := range ledger.Staged {
		// Ensure absolute paths for overlay
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		overlay[absPath] = []byte(patch.NewContent)
	}

	cfg := &packages.Config{
		Mode:    packages.NeedTypes | packages.NeedSyntax | packages.NeedDeps | packages.NeedImports | packages.NeedName,
		Overlay: overlay,
		Dir:     v.ProjectRoot,
		Tests:   true,
		Env:     append(os.Environ(), "CGO_ENABLED=0"),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return ValidationResult{}, fmt.Errorf("failed to load packages for LSP validation: %w", err)
	}

	var violations []string
	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			// We only care about type errors that indicate broken contracts
			// e.g., "does not implement", "missing method", "wrong signature"
			msg := pkgErr.Msg
			if strings.Contains(msg, "does not implement") ||
				strings.Contains(msg, "missing method") ||
				strings.Contains(msg, "have") ||
				strings.Contains(msg, "want") {
				violations = append(violations, fmt.Sprintf("[%s] %s", pkg.PkgPath, msg))
			}
		}
	}

	if len(violations) > 0 {
		return ValidationResult{
			Valid:   false,
			Message: "Liskov Substitution Principle (LSP) violations detected",
			Details: map[string]any{"violations": violations},
		}, nil
	}

	return ValidationResult{Valid: true, Message: "LSP verification successful"}, nil
}

// Propagate traces the blast radius of a symbol and applies the transformation to all affected files.
func (e *RippleEngine) Propagate(ctx context.Context, symbolName string, transformation string, maxDepth int) (*Ledger, error) {
	ledger := NewLedger()
	// Initialize budget from config or defaults
	ledger.SetBudget(100000, 15) // Example: 100k Ki, 15 turns

	strategy := e.Strategy
	if strategy == nil {
		strategy = NewBFSPropagationStrategy(e.store, e.ImpactEngine)
	}

	// 1. Stage changes
	for task, err := range strategy.Discover(ctx, symbolName, maxDepth) {
		if err != nil {
			return nil, err
		}

		if _, exists := ledger.Staged[task.FilePath]; !exists {
			ledger.IncrementTurn() // Each transformation is a turn

			newContent, err := e.Transformer.Transform(ctx, task.FilePath, task.SymbolName, transformation)
			if err != nil {
				return nil, fmt.Errorf("transformation failed for %s: %w", task.FilePath, err)
			}

			if err := ledger.Stage(task.FilePath, Patch{
				FilePath:   task.FilePath,
				NewContent: newContent,
			}); err != nil {
				return ledger, err // Return ledger even on budget error so user can see partial progress
			}
		}
	}

	// 2. Validate pipeline
	for _, v := range e.Validators {
		res, err := v.Validate(ctx, ledger)
		if err != nil {
			return nil, fmt.Errorf("validator error: %w", err)
		}
		if !res.Valid {
			return ledger, fmt.Errorf("validation failed: %s", res.Message)
		}
	}

	return ledger, nil
}

// MCPTransformer implements Transformer using MCP Sampling.
type MCPTransformer struct {
	store       GraphStore
	DoTransform func(ctx context.Context, file, symbol, transformation string) (string, error)
}

func NewMCPTransformer(s GraphStore, bridge func(context.Context, string, string, string) (string, error)) *MCPTransformer {
	return &MCPTransformer{
		store:       s,
		DoTransform: bridge,
	}
}

func (t *MCPTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file for transformation context: %w", err)
	}

	// Build a rich contextual prompt for the LLM
	contextualPrompt := fmt.Sprintf(`Act as a senior software architect. Transform the following code based on the instruction.

FILE: %s
SYMBOL TO MODIFY: %s
INSTRUCTION: %s

CODE:
%s`, file, symbol, transformation, string(content))

	return t.DoTransform(ctx, file, symbol, contextualPrompt)
}

// StructuralTransformer implements Transformer using structural search and replace.
type StructuralTransformer struct {
	Pattern string
}

func (t *StructuralTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	ext := filepath.Ext(file)
	pattern := strings.ReplaceAll(t.Pattern, "$SYMBOL", symbol)
	return StructuralRefactor(ctx, file, pattern, transformation, ext)
}
