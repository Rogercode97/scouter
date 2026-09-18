package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

type mockContextStore struct {
	GraphStore
	symbols       map[string][]store.Symbol
	callers       map[string][]store.Call
	recCallers    map[string][]store.Call
	affectedTests map[string][]store.Symbol
}

func (m *mockContextStore) GetSymbolsByPathPrefix(ctx context.Context, prefix string) ([]store.Symbol, error) {
	return m.symbols[prefix], nil
}

func (m *mockContextStore) GetCallers(ctx context.Context, calleeName string, limit, offset int) ([]store.Call, error) {
	return m.callers[calleeName], nil
}

func (m *mockContextStore) GetCallersRecursive(ctx context.Context, name, path string, maxDepth int) ([]store.Call, error) {
	return m.recCallers[name], nil
}

func (m *mockContextStore) GetAffectedTestsRecursive(ctx context.Context, name, path string) ([]store.Symbol, error) {
	return m.affectedTests[name], nil
}

func TestContextEngine_BuildFileContext(t *testing.T) {
	ctx := context.Background()

	// Create temporary dummy file to test LOC calculation
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "service.go")
	testFile := filepath.Join(tmpDir, "service_test.go")

	code := "package test\n\nfunc Process() {\n\t// line 4\n\t// line 5\n}\n"
	if err := os.WriteFile(dummyFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("package test\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	mockStore := &mockContextStore{
		symbols: map[string][]store.Symbol{
			dummyFile: {
				{
					Name:       "Process",
					Path:       dummyFile,
					StartLine:  3,
					EndLine:    6,
					ChurnScore: 0.45,
				},
			},
		},
		callers: map[string][]store.Call{
			"Process": {
				{CallerName: "HandleRequest", Path: filepath.Join(tmpDir, "handler.go")},
				{CallerName: "CronJob", Path: filepath.Join(tmpDir, "cron.go")},
			},
		},
		recCallers: map[string][]store.Call{
			"Process": {
				{CallerName: "HandleRequest", Path: filepath.Join(tmpDir, "handler.go")},
				{CallerName: "Router", Path: filepath.Join(tmpDir, "router.go")},
				{CallerName: "CronJob", Path: filepath.Join(tmpDir, "cron.go")},
			},
		},
		affectedTests: map[string][]store.Symbol{
			"Process": {
				{Name: "TestProcess", Path: testFile},
			},
		},
	}

	engine := NewContextEngine(mockStore)
	res, err := engine.BuildFileContext(ctx, dummyFile)
	if err != nil {
		t.Fatalf("BuildFileContext returned error: %v", err)
	}

	if res.File != dummyFile {
		t.Errorf("expected file %s, got %s", dummyFile, res.File)
	}
	if res.LOC != 6 {
		t.Errorf("expected LOC 6, got %d", res.LOC)
	}
	if res.Symbols != 1 {
		t.Errorf("expected 1 symbol, got %d", res.Symbols)
	}
	if len(res.DirectDependents) != 2 {
		t.Errorf("expected 2 direct dependents, got %d", len(res.DirectDependents))
	}
	if res.TransitiveImpact != 3 {
		t.Errorf("expected 3 transitive impact files, got %d", res.TransitiveImpact)
	}
	if res.EditCost.Files != 2 {
		t.Errorf("expected edit cost files 2, got %d", res.EditCost.Files)
	}
	if res.EditCost.Cascade != 3 {
		t.Errorf("expected edit cost cascade 3, got %d", res.EditCost.Cascade)
	}
	if res.EditCost.Risk != "medium" {
		t.Errorf("expected risk 'medium', got %s", res.EditCost.Risk)
	}
	if res.Verdict != "caution" {
		t.Errorf("expected verdict 'caution', got %s", res.Verdict)
	}
	if len(res.TestHints) == 0 || res.TestHints[0] != testFile {
		t.Errorf("expected test hints containing %s, got %v", testFile, res.TestHints)
	}
}
