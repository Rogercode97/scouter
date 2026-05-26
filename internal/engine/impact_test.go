package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

type mockImpactStore struct {
	GraphStore
	symbolsByRange map[string][]store.Symbol
	affectedTests  map[string][]store.Symbol
}

// store.SymbolRegistry methods
func (m *mockImpactStore) SaveSymbol(ctx context.Context, sym *store.Symbol) error { return nil }
func (m *mockImpactStore) SearchSymbols(ctx context.Context, query, fileType string, limit, offset int) ([]store.Symbol, error) { return nil, nil }
func (m *mockImpactStore) DeleteSymbolsByFile(ctx context.Context, path string) error { return nil }
func (m *mockImpactStore) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]store.Symbol, error) {
	return m.symbolsByRange[path], nil
}
func (m *mockImpactStore) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]store.Symbol, error) {
	return []store.Symbol{{Name: name, Path: path}}, nil
}

// store.StructuralGraph methods
func (m *mockImpactStore) SaveCall(ctx context.Context, call store.Call) error { return nil }
func (m *mockImpactStore) DeleteCallsByFile(ctx context.Context, path string) error { return nil }
func (m *mockImpactStore) GetCallersRecursive(ctx context.Context, symbol string, path string, maxDepth int) ([]store.Call, error) {
	return []store.Call{}, nil
}
func (m *mockImpactStore) GetAffectedTestsRecursive(ctx context.Context, symbol, path string) ([]store.Symbol, error) {
	return m.affectedTests[symbol+":"+path], nil
}

func TestImpactEngine_Analyze(t *testing.T) {
	ctx := context.Background()
	db := &mockImpactStore{}
	engine := &ImpactEngine{
		store: db,
	}

	res, err := engine.Analyze(ctx, "MySymbol", "main.go", 3)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if res.Target.Symbol != "MySymbol" {
		t.Errorf("expected symbol MySymbol, got %s", res.Target.Symbol)
	}
}

func TestImpactEngine_PredictTests(t *testing.T) {
	path, _ := filepath.Abs("internal/engine/processor.go")
	testPath, _ := filepath.Abs("internal/engine/processor_test.go")

	db := &mockImpactStore{
		symbolsByRange: map[string][]store.Symbol{
			path: {
				{Name: "ProcessData", Path: path},
			},
		},
		affectedTests: map[string][]store.Symbol{
			"ProcessData:" + path: {
				{Name: "TestProcessData", Path: testPath},
			},
		},
	}

	diff := `--- a/internal/engine/processor.go
+++ b/internal/engine/processor.go
@@ -10,1 +10,1 @@
-func ProcessData() {
+func ProcessData(ctx context.Context) {`

	ctx := context.Background()
	engine := &ImpactEngine{store: db}
	tests, err := engine.PredictTests(ctx, diff)
	if err != nil {
		t.Fatalf("PredictTests failed: %v", err)
	}

	if len(tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(tests))
	}

	if tests[0].Name != "TestProcessData" || tests[0].File != testPath {
		t.Errorf("unexpected test: %+v", tests[0])
	}
}

func TestImpactEngine_PredictTestsEmptyDiff(t *testing.T) {
	engine := &ImpactEngine{store: &mockImpactStore{}}
	tests, err := engine.PredictTests(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 0 {
		t.Error("expected empty result")
	}
}

func TestFindTestsForSymbolsUnique(t *testing.T) {
	db := &mockImpactStore{
		affectedTests: map[string][]store.Symbol{
			"A:file.go": {
				{Name: "Test1", Path: "test.go"},
			},
			"B:file.go": {
				{Name: "Test1", Path: "test.go"},
				{Name: "Test2", Path: "test.go"},
			},
		},
	}

	symbols := []store.Symbol{
		{Name: "A", Path: "file.go"},
		{Name: "B", Path: "file.go"},
	}

	tests, err := findTestsForSymbols(context.Background(), db, symbols)
	if err != nil {
		t.Fatal(err)
	}

	if len(tests) != 2 {
		t.Errorf("expected 2 unique tests, got %d", len(tests))
	}

	// Verify TestTarget usage
	var _ types.TestTarget = tests[0]
}
