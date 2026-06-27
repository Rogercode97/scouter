package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Rogercode97/scouter/internal/types"
)

// DiagnosticContext contains everything needed to propose a fix.
type DiagnosticContext struct {
	FailingFile  string
	ErrorLog     string
	OriginalCode string
	Prompt       string
	Target       *types.ASTPointer
}

// FixCandidate represents a potential fix.
type FixCandidate struct {
	ID       int
	Code     string
	FullCode string
	Valid    bool
	Score    float64
}

// Solver is the generative interface (LLM/AI).
type Solver interface {
	Generate(ctx context.Context, diag *DiagnosticContext) (<-chan FixCandidate, error)
}

// Verifier evaluates a candidate's validity (LSP/AST).
type Verifier interface {
	Evaluate(ctx context.Context, diag *DiagnosticContext, c *FixCandidate) (bool, error)
}

// ShinigamiPipeline orchestrates the Solver and Verifier.
type ShinigamiPipeline struct {
	solver   Solver
	verifier Verifier
}

func NewShinigamiPipeline(s Solver, v Verifier) *ShinigamiPipeline {
	return &ShinigamiPipeline{solver: s, verifier: v}
}

func (p *ShinigamiPipeline) Run(ctx context.Context, diag *DiagnosticContext) (*FixCandidate, error) {
	evalCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	candidatesCh, err := p.solver.Generate(evalCtx, diag)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var bestCandidate *FixCandidate
	var lastErr error

	for c := range candidatesCh {
		wg.Add(1)
		go func(candidate FixCandidate) {
			defer wg.Done()

			valid, vErr := p.verifier.Evaluate(evalCtx, diag, &candidate)

			mu.Lock()
			defer mu.Unlock()

			if vErr != nil && !errors.Is(vErr, context.Canceled) {
				lastErr = vErr
				return
			}

			if valid {
				candidate.Score = 1.0
				// Basic penalty for massive code changes could go here

				if bestCandidate == nil || candidate.Score > bestCandidate.Score || (candidate.Score == bestCandidate.Score && candidate.ID < bestCandidate.ID) {
					bestCandidate = &candidate
				}
				cancel() // Stop further evaluations once we have a valid one
			}
		}(c)
	}

	wg.Wait()

	if evalCtx.Err() != nil && errors.Is(evalCtx.Err(), context.DeadlineExceeded) {
		return nil, evalCtx.Err()
	}

	if bestCandidate == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("all candidates failed validation: %w", lastErr)
		}
		return nil, errors.New("all candidates failed validation")
	}

	return bestCandidate, nil
}
