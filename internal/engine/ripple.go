package engine

import (
	"context"
	"fmt"

	"github.com/Rogercode97/scouter/internal/store"
)

// Transformer defines how to modify a file based on a symbolic change.
type Transformer interface {
	Transform(ctx context.Context, file string, symbolName string, transformation string) (string, error)
}

// RippleEngine orchestrates multi-file symbolic refactoring.
type RippleEngine struct {
	store       store.Repository
	Transformer Transformer
}

func NewRippleEngine(s store.Repository, t Transformer) *RippleEngine {
	return &RippleEngine{
		store:       s,
		Transformer: t,
	}
}

// Propagate traces the blast radius of a symbol and applies the transformation to all affected files.
func (e *RippleEngine) Propagate(ctx context.Context, symbolName string, transformation string, maxDepth int) (*Ledger, error) {
	ledger := NewLedger()
	visited := make(map[string]bool)
	queue := []string{symbolName}
	
	depth := 0
	for len(queue) > 0 && depth < maxDepth {
		nextQueue := []string{}
		for _, currentSym := range queue {
			if visited[currentSym] {
				continue
			}
			visited[currentSym] = true

			// 1. Trace callers via Global Call Graph
			callers, err := e.store.GetCallers(ctx, currentSym)
			if err != nil {
				return nil, fmt.Errorf("failed to get callers for %s: %w", currentSym, err)
			}

			for _, caller := range callers {
				// Record files that need transformation
				if _, exists := ledger.patches[caller.Path]; !exists {
					newContent, err := e.Transformer.Transform(ctx, caller.Path, currentSym, transformation)
					if err != nil {
						return nil, fmt.Errorf("transformation failed for %s: %w", caller.Path, err)
					}
					ledger.Record(caller.Path, Patch{
						FilePath:   caller.Path,
						NewContent: newContent,
					})
				}
				
				// Add the caller to the queue for next depth ripple
				nextQueue = append(nextQueue, caller.CallerName)
			}

			// 2. Also include the symbol definition file itself (Depth 0)
			results, _ := e.store.SearchSymbols(ctx, currentSym, "")
			for _, sym := range results {
				if sym.Name == currentSym {
					if _, exists := ledger.patches[sym.Path]; !exists {
						newContent, err := e.Transformer.Transform(ctx, sym.Path, currentSym, transformation)
						if err != nil {
							return nil, fmt.Errorf("transformation failed for definition in %s: %w", sym.Path, err)
						}
						ledger.Record(sym.Path, Patch{
							FilePath:   sym.Path,
							NewContent: newContent,
						})
					}
				}
			}
		}
		queue = nextQueue
		depth++
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
