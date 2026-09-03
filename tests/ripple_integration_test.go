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

func (m *mockRippleStore) SearchSymbols(ctx context.Context, query, symType, pathPrefix string, limit, offset int) ([]store.Symbol, error) {
	return m.symbols, nil
}

func (m *mockRippleStore) GetRippleGraphRecursive(ctx context.Context, startSymbol string, maxDepth int) ([]store.Call, error) {
	var edges []store.Call
	for _, c := range m.callers {
		if c.CalleeName == "" {
			c.CalleeName = startSymbol
		}
		edges = append(edges, c)
	}
	return edges, nil
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

	t.Run("Commit Failure Rollback Preserves Modes And Cleans Tmp", func(t *testing.T) {
		// Reset files with explicit modes
		modeA := os.FileMode(0755)
		modeB := os.FileMode(0644)
		if err := os.WriteFile(fileA, []byte(contentA), modeA); err != nil {
			t.Fatalf("failed to write fileA: %v", err)
		}
		if err := os.Chmod(fileA, modeA); err != nil {
			t.Fatalf("failed to chmod fileA: %v", err)
		}
		if err := os.WriteFile(fileB, []byte(contentB), modeB); err != nil {
			t.Fatalf("failed to write fileB: %v", err)
		}
		if err := os.Chmod(fileB, modeB); err != nil {
			t.Fatalf("failed to chmod fileB: %v", err)
		}

		re.Validators = nil // clear validators
		ledger, err := re.Propagate(ctx, "SymA", "rename", 1)
		if err != nil {
			t.Fatalf("Propagate failed: %v", err)
		}

		// Stage a third patch targeting an invalid path (existing directory) to guarantee apply failure
		badDir := filepath.Join(tempDir, "bad_dir_target")
		if err := os.MkdirAll(badDir, 0755); err != nil {
			t.Fatalf("failed to create badDir: %v", err)
		}
		if err := ledger.Stage(badDir, engine.Patch{
			FilePath:   badDir,
			NewContent: "will fail on rename",
		}); err != nil {
			t.Fatalf("failed to stage badDir: %v", err)
		}

		err = ledger.CommitStaged(ctx)
		if err == nil {
			t.Fatalf("expected CommitStaged to fail due to badDir collision")
		}

		// Verify existing files are preserved with content and mode
		statA, err := os.Stat(fileA)
		if err != nil {
			t.Fatalf("fileA was deleted: %v", err)
		}
		if statA.Mode() != modeA {
			t.Errorf("expected fileA mode %v, got %v", modeA, statA.Mode())
		}
		dataA, err := os.ReadFile(fileA)
		if err != nil {
			t.Fatalf("failed to read fileA: %v", err)
		}
		if string(dataA) != contentA {
			t.Errorf("fileA content not restored, got %q, want %q", string(dataA), contentA)
		}

		statB, err := os.Stat(fileB)
		if err != nil {
			t.Fatalf("fileB was deleted: %v", err)
		}
		if statB.Mode() != modeB {
			t.Errorf("expected fileB mode %v, got %v", modeB, statB.Mode())
		}
		dataB, err := os.ReadFile(fileB)
		if err != nil {
			t.Fatalf("failed to read fileB: %v", err)
		}
		if string(dataB) != contentB {
			t.Errorf("fileB content not restored, got %q, want %q", string(dataB), contentB)
		}

		// Verify no temporary files remain
		for _, p := range []string{fileA, fileB, badDir} {
			tmp := p + ".scouter.tmp"
			if _, err := os.Stat(tmp); !os.IsNotExist(err) {
				t.Errorf("temporary staging file leaked: %s", tmp)
			}
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
