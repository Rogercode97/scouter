package engine

import (
	"context"
	"iter"
	"os"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestMCPTransformer_Transform(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "scouter_test_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := "package main\n\nfunc Test() {}"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	ms := &rippleEngineMockStore{}

	bridgeCalled := false
	var capturedPrompt string
	bridge := func(ctx context.Context, file, symbol, prompt string) (string, error) {
		bridgeCalled = true
		capturedPrompt = prompt
		return "transformed code", nil
	}

	transformer := NewMCPTransformer(ms, bridge)

	res, err := transformer.Transform(context.Background(), tmpFile.Name(), "Test", "rename to NewTest")
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if res != "transformed code" {
		t.Errorf("Expected 'transformed code', got %s", res)
	}

	if !bridgeCalled {
		t.Error("Bridge function was not called")
	}

	if !strings.Contains(capturedPrompt, "SYMBOL TO MODIFY: Test") {
		t.Error("Prompt does not contain symbol name")
	}

	if !strings.Contains(capturedPrompt, "CODE:\npackage main") {
		t.Error("Prompt does not contain file content")
	}
}

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

type rippleEngineMockStore struct {
	GraphStore
	callers map[string][]store.Call
	callees map[string][]store.Call
	symbols map[string][]store.Symbol
}

func (m *rippleEngineMockStore) GetCallers(ctx context.Context, name string, limit, offset int) ([]store.Call, error) {
	return m.callers[name], nil
}

func (m *rippleEngineMockStore) GetCallees(ctx context.Context, name string) ([]store.Call, error) {
	return m.callees[name], nil
}

func (m *rippleEngineMockStore) SearchSymbols(ctx context.Context, query, symType string, limit, offset int) ([]store.Symbol, error) {
	return m.symbols[query], nil
}

func (m *rippleEngineMockStore) GetAllSymbols(ctx context.Context) iter.Seq2[store.Symbol, error] {
	return func(yield func(store.Symbol, error) bool) {
		for _, syms := range m.symbols {
			for _, s := range syms {
				if !yield(s, nil) {
					return
				}
			}
		}
	}
}

func TestRippleEngine_Propagate(t *testing.T) {
	ms := &rippleEngineMockStore{
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

	strategy := &BFSPropagationStrategy{store: ms, ImpactEngine: ie}
	
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

		if len(ledger.Staged) != 2 {
			t.Errorf("Expected 2 staged files, got %d", len(ledger.Staged))
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
	ms := &rippleEngineMockStore{
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
		if !found["SymA"] {
			t.Errorf("Expected to find SymA at depth 0, got %v", found)
		}
		if len(found) != 1 {
			t.Errorf("Expected 1 unique symbol at depth 0, got %d", len(found))
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
		// Depth 0: SymA
		// Depth 1: SymA (for callers of SymA)
		if !found["SymA"] {
			t.Errorf("Expected to find SymA at depth 1, got %v", found)
		}
	})
}
