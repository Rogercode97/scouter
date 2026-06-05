package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
)

type mockTruthTransformer struct {
	transformFunc func(file, symbol, transformation string) (string, error)
}

func (t *mockTruthTransformer) Transform(ctx context.Context, file, symbol, transformation string) (string, error) {
	return t.transformFunc(file, symbol, transformation)
}

type mockRippleStore struct {
	store.Store
	callers []store.Call
	callees []store.Call
	symbols []store.Symbol
}

func (m *mockRippleStore) GetCallers(ctx context.Context, name string, limit, offset int) ([]store.Call, error) {
	return m.callers, nil
}

func (m *mockRippleStore) GetCallees(ctx context.Context, name string) ([]store.Call, error) {
	return m.callees, nil
}

func (m *mockRippleStore) GetCallersRecursive(ctx context.Context, name string, path string, maxDepth int) ([]store.Call, error) {
	return m.callers, nil
}

func (m *mockRippleStore) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]store.Symbol, error) {
	return m.symbols, nil
}

func (m *mockRippleStore) SearchSymbols(ctx context.Context, query, symType string, limit, offset int) ([]store.Symbol, error) {
	return m.symbols, nil
}

func TestRippleIntegration_FullFlow(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "ripple-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fileA := filepath.Join(tempDir, "fileA.go")
	contentA := "package main\n\nfunc SymA() {}"
	os.WriteFile(fileA, []byte(contentA), 0644)

	fileB := filepath.Join(tempDir, "fileB.go")
	contentB := "package main\n\nfunc SymB() { SymA() }"
	os.WriteFile(fileB, []byte(contentB), 0644)

	ms := &mockRippleStore{
		callers: []store.Call{
			{CallerName: "SymB", Path: fileB},
		},
		symbols: []store.Symbol{
			{Name: "SymA", Path: fileA},
		},
	}

	ie := engine.NewImpactEngine(ms, nil, nil)
	mt := &mockTruthTransformer{
		transformFunc: func(file, symbol, transformation string) (string, error) {
			return "transformed " + symbol + " in " + file, nil
		},
	}

	strategy := engine.NewBFSPropagationStrategy(ms, ie)
	re := engine.NewRippleEngine(ms, mt, ie)
	re.Strategy = strategy

	t.Run("Commit Flow", func(t *testing.T) {
		ledger, err := re.Propagate(ctx, "SymA", "rename", 1)
		if err != nil {
			t.Fatalf("Propagate failed: %v", err)
		}

		if len(ledger.StagedFiles()) != 2 {
			t.Errorf("Expected 2 staged files, got %d", len(ledger.StagedFiles()))
		}

		err = ledger.CommitStaged(ctx)
		if err != nil {
			t.Fatalf("CommitStaged failed: %v", err)
		}

		// Verify file content changed
		newA, _ := os.ReadFile(fileA)
		if string(newA) != "transformed SymA in "+fileA {
			t.Errorf("fileA not updated correctly, got: %s", string(newA))
		}
	})

	t.Run("Rollback on Validation Failure", func(t *testing.T) {
		// Reset files
		os.WriteFile(fileA, []byte(contentA), 0644)
		os.WriteFile(fileB, []byte(contentB), 0644)

		mv := &mockValidator{
			validateFunc: func(ledger *engine.Ledger) (engine.ValidationResult, error) {
				return engine.ValidationResult{Valid: false, Message: "Build error"}, nil
			},
		}
		re.Validators = []engine.Validator{mv}

		ledger, err := re.Propagate(ctx, "SymA", "rename", 1)
		if err == nil || err.Error() != "validation failed: Build error" {
			t.Errorf("Expected validation failure, got: %v", err)
		}

		// Rollback manually (usually TruthEngine handles this)
		err = ledger.Rollback(ctx)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify files restored
		restoredA, _ := os.ReadFile(fileA)
		if string(restoredA) != contentA {
			t.Errorf("fileA not restored, got: %s", string(restoredA))
		}
	})
}

type mockValidator struct {
	validateFunc func(ledger *engine.Ledger) (engine.ValidationResult, error)
}

func (v *mockValidator) Validate(ctx context.Context, ledger *engine.Ledger) (engine.ValidationResult, error) {
	return v.validateFunc(ledger)
}

func TestImpactEngine_Analyze_Mixed(t *testing.T) {
	ctx := context.Background()

	// Setup mock store with Go and Rust symbols
	ms := &mockRippleStore{
		callers: []store.Call{
			{CallerName: "SymB", Path: "fileB.go", LinkType: "calls"},
			{CallerName: "Dog", Path: "dog.rs", LinkType: "implements"},
		},
		symbols: []store.Symbol{
			{Name: "SymA", Path: "fileA.go", Pagerank: 42.0},
		},
	}

	ie := engine.NewImpactEngine(ms, nil, nil)

	res, err := ie.Analyze(ctx, "SymA", "fileA.go", 1)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Verify Centrality is using Pagerank
	if res.Target.Metrics.Centrality != 42.0 {
		t.Errorf("Expected Centrality 42.0, got %f", res.Target.Metrics.Centrality)
	}

	// Verify Mermaid generation
	if res.Mermaid == "" {
		t.Errorf("Expected Mermaid graph, got empty")
	}

	// Check for correct edges
	// calls edge should be -->
	// implements edge should be -.->
	mermaid := res.Mermaid
	if !contains(mermaid, "-->") {
		t.Errorf("Expected --> in mermaid graph, got: %s", mermaid)
	}
	if !contains(mermaid, "-.->") {
		t.Errorf("Expected -.-> in mermaid graph, got: %s", mermaid)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
