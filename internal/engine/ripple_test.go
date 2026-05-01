package engine

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

type mockTransformer struct {
	transformFunc func(file, symbol, transformation string) (string, error)
}

func (t *mockTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	return t.transformFunc(file, symbol, transformation)
}

type mockValidator struct {
	validateFunc func(ledger *Ledger) (ValidationResult, error)
}

func (v *mockValidator) Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error) {
	return v.validateFunc(ledger)
}

func TestRippleEngine_Propagate(t *testing.T) {
	ms := &rippleMockStore{
		callers: map[string][]store.Call{
			"SymA": {{CallerName: "SymB", Path: "fileB.go"}},
		},
		symbols: map[string][]store.Symbol{
			"SymA": {{Name: "SymA", Path: "fileA.go"}},
		},
	}
	ie := &ImpactEngine{store: ms}
	
	mt := &mockTransformer{
		transformFunc: func(file, symbol, transformation string) (string, error) {
			return "transformed " + file, nil
		},
	}

	strategy := &BFSPropagationStrategy{store: ms, impactEngine: ie}
	
	engine := &RippleEngine{
		store:        ms,
		Transformer:  mt,
		ImpactEngine: ie,
		Strategy:     strategy,
	}

	ctx := context.Background()

	t.Run("Successful Propagation", func(t *testing.T) {
		ledger, err := engine.Propagate(ctx, "SymA", "rename", 1)
		if err != nil {
			t.Fatalf("Propagate failed: %v", err)
		}

		if len(ledger.staged) != 2 {
			t.Errorf("Expected 2 staged files, got %d", len(ledger.staged))
		}
	})

	t.Run("Validation Failure", func(t *testing.T) {
		mv := &mockValidator{
			validateFunc: func(ledger *Ledger) (ValidationResult, error) {
				return ValidationResult{Valid: false, Message: "Build error"}, nil
			},
		}
		engine.Validators = []Validator{mv}

		ledger, err := engine.Propagate(ctx, "SymA", "rename", 1)
		if err == nil {
			t.Fatal("Expected propagation to fail due to validation, but it succeeded")
		}

		if err.Error() != "validation failed: Build error" {
			t.Errorf("Unexpected error message: %v", err)
		}

		if ledger == nil {
			t.Fatal("Expected ledger to be returned even on validation failure")
		}
	})
}

func TestBFSPropagationStrategy_Depth(t *testing.T) {
	ms := &rippleMockStore{
		callers: map[string][]store.Call{
			"SymA": {{CallerName: "SymB", Path: "fileB.go"}},
			"SymB": {{CallerName: "SymC", Path: "fileC.go"}},
			"SymC": {{CallerName: "SymD", Path: "fileD.go"}},
		},
		symbols: map[string][]store.Symbol{
			"SymA": {{Name: "SymA", Path: "fileA.go"}},
			"SymB": {{Name: "SymB", Path: "fileB.go"}},
			"SymC": {{Name: "SymC", Path: "fileC.go"}},
		},
	}
	ie := &ImpactEngine{store: ms}
	strategy := NewBFSPropagationStrategy(ms, ie)
	ctx := context.Background()

	t.Run("Depth 0", func(t *testing.T) {
		found := make(map[string]bool)
		for task, err := range strategy.Discover(ctx, "SymA", 0) {
			if err != nil {
				t.Fatalf("Discover failed: %v", err)
			}
			found[task.SymbolName] = true
		}
		if !found["SymA"] || !found["SymB"] {
			t.Errorf("Expected to find SymA and SymB at depth 0, got %v", found)
		}
		if len(found) != 2 {
			t.Errorf("Expected 2 unique symbols at depth 0, got %d", len(found))
		}
	})

	t.Run("Depth 1", func(t *testing.T) {
		found := make(map[string]bool)
		for task, err := range strategy.Discover(ctx, "SymA", 1) {
			if err != nil {
				t.Fatalf("Discover failed: %v", err)
			}
			found[task.SymbolName] = true
		}
		// Depth 0: SymA -> SymB, SymA
		// Depth 1: SymB -> SymC, SymB
		if !found["SymA"] || !found["SymB"] || !found["SymC"] {
			t.Errorf("Expected to find SymA, SymB, and SymC at depth 1, got %v", found)
		}
		if len(found) != 3 {
			t.Errorf("Expected 3 unique symbols at depth 1, got %d", len(found))
		}
	})
}
