package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

type predictMockStore struct {
	store.Repository
	symbolsByRange map[string][]store.Symbol
	affectedTests  map[string][]store.Symbol
}

func (m *predictMockStore) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]store.Symbol, error) {
	return m.symbolsByRange[path], nil
}

func (m *predictMockStore) GetAffectedTests(ctx context.Context, symbol, path string) ([]store.Symbol, error) {
	return m.affectedTests[symbol+":"+path], nil
}

func TestPredictTests(t *testing.T) {
	path, _ := filepath.Abs("internal/engine/processor.go")
	testPath, _ := filepath.Abs("internal/engine/processor_test.go")

	db := &predictMockStore{
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
	tests, err := PredictTests(ctx, db, diff)
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

func TestPredictTestsEmptyDiff(t *testing.T) {
	tests, err := PredictTests(context.Background(), &predictMockStore{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 0 {
		t.Error("expected empty result")
	}
}

func TestFindTestsForSymbolsUnique(t *testing.T) {
	db := &predictMockStore{
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
