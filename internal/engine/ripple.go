package engine

import (
	"context"
	"fmt"
	"iter"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/store"
)

// PropagationTask represents a single unit of work in the ripple propagation.
type PropagationTask struct {
	SymbolName string
	FilePath   string
	Action     string
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

// RippleEngine orchestrates multi-file symbolic refactoring.
type RippleEngine struct {
	store        store.Repository
	Transformer  Transformer
	ImpactEngine *ImpactEngine
	Strategy     PropagationStrategy
	Validators   []Validator
}

func NewRippleEngine(s store.Repository, t Transformer, ie *ImpactEngine) *RippleEngine {
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
	store        store.Repository
	impactEngine *ImpactEngine
}

func NewBFSPropagationStrategy(s store.Repository, ie *ImpactEngine) *BFSPropagationStrategy {
	return &BFSPropagationStrategy{
		store:        s,
		impactEngine: ie,
	}
}

func (s *BFSPropagationStrategy) Discover(ctx context.Context, startSymbol string, maxDepth int) iter.Seq2[PropagationTask, error] {
	return func(yield func(PropagationTask, error) bool) {
		visited := make(map[string]bool)
		queue := []string{startSymbol}

		depth := 0
		for len(queue) > 0 && depth <= maxDepth {
			nextQueue := []string{}
			for _, currentSym := range queue {
				if visited[currentSym] {
					continue
				}
				visited[currentSym] = true

				// 1. Trace callers via Hybrid Determinism
				var callers []store.Call
				deterministic, err := s.impactEngine.GetDeterministicCallers(ctx, currentSym)
				if err == nil && len(deterministic) > 0 {
					callers = deterministic
				} else {
					// Fallback to Global Call Graph (heuristic)
					callers, err = s.store.GetCallers(ctx, currentSym, 0, 0)
					if err != nil {
						if !yield(PropagationTask{}, fmt.Errorf("failed to get callers for %s: %w", currentSym, err)) {
							return
						}
						continue
					}
				}

				for _, caller := range callers {
					if !yield(PropagationTask{
						SymbolName: caller.CallerName,
						FilePath:   caller.Path,
						Action:     "transform",
					}, nil) {
						return
					}
					nextQueue = append(nextQueue, caller.CallerName)
				}

				// 2. Also include the symbol definition file itself (Depth 0)
				results, _ := s.store.SearchSymbols(ctx, currentSym, "", 0, 0)
				for _, sym := range results {
					if sym.Name == currentSym {
						if !yield(PropagationTask{
							SymbolName: sym.Name,
							FilePath:   sym.Path,
							Action:     "transform",
						}, nil) {
							return
						}
					}
				}
			}
			queue = nextQueue
			depth++
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

// Propagate traces the blast radius of a symbol and applies the transformation to all affected files.
func (e *RippleEngine) Propagate(ctx context.Context, symbolName string, transformation string, maxDepth int) (*Ledger, error) {
	ledger := NewLedger()
	strategy := e.Strategy
	if strategy == nil {
		strategy = NewBFSPropagationStrategy(e.store, e.ImpactEngine)
	}

	// 1. Stage changes
	for task, err := range strategy.Discover(ctx, symbolName, maxDepth) {
		if err != nil {
			return nil, err
		}

		if _, exists := ledger.staged[task.FilePath]; !exists {
			newContent, err := e.Transformer.Transform(ctx, task.FilePath, task.SymbolName, transformation)
			if err != nil {
				return nil, fmt.Errorf("transformation failed for %s: %w", task.FilePath, err)
			}
			ledger.Stage(task.FilePath, Patch{
				FilePath:   task.FilePath,
				NewContent: newContent,
			})
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
	// This will be bridged from the MCP handler
	DoTransform func(ctx context.Context, file, symbol, transformation string) (string, error)
}

func (t *MCPTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	return t.DoTransform(ctx, file, symbol, transformation)
}
