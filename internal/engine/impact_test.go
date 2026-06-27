package engine

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

type mockImpactStore struct {
	GraphStore
	symbolsByRange map[string][]store.Symbol
	affectedTests  map[string][]store.Symbol
	SearchSymbolsFunc func(ctx context.Context, query, fileType, pathPrefix string, limit, offset int) ([]store.Symbol, error)
}

// store.SymbolRegistry methods
func (m *mockImpactStore) SaveSymbol(ctx context.Context, sym *store.Symbol) error { return nil }
func (m *mockImpactStore) SearchSymbols(ctx context.Context, query, fileType, pathPrefix string, limit, offset int) ([]store.Symbol, error) {
	if m.SearchSymbolsFunc != nil {
		return m.SearchSymbolsFunc(ctx, query, fileType, pathPrefix, limit, offset)
	}
	return nil, nil
}
func (m *mockImpactStore) DeleteSymbolsByFile(ctx context.Context, path string) error { return nil }
func (m *mockImpactStore) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]store.Symbol, error) {
	return m.symbolsByRange[path], nil
}
func (m *mockImpactStore) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]store.Symbol, error) {
	return []store.Symbol{{Name: name, Path: path}}, nil
}

// store.StructuralGraph methods
func (m *mockImpactStore) SaveCall(ctx context.Context, call store.Call) error      { return nil }
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

	// We expect risk score to be updated using new formula
	// bScore = log(1+0)/log(1+500) = 0
	// cogComplexity = 0 -> cogScore = 0
	// churnScore = 0
	// testGap = 1.0 (since no main_test.go exists)
	// volumeScore = 0
	// runtimeScore = 0
	// New Formula: (bScore*0.20) + (cogScore*0.35) + (churnScore*0.15) + (testGap*0.15) + (volumeScore*0.05) + (runtimeScore*0.10)
	// RiskScore should be exactly 0.15.
	if res.Target.RiskScore != 0.15 {
		t.Errorf("expected RiskScore 0.15, got %f", res.Target.RiskScore)
	}
}

func TestImpactEngine_RiskScoreFormula(t *testing.T) {
	ctx := context.Background()

	// Create mock that returns specific metadata for formula
	db := &mockSpecificImpactStore{
		mockImpactStore: &mockImpactStore{
			affectedTests: make(map[string][]store.Symbol),
		},
	}

	mockMem := &testMemoryProvider{
		searchInsights: []types.MemoryInsight{
			{ID: "1", Type: "bugfix", Title: "fix 1"},
			{ID: "2", Type: "bugfix", Title: "fix 2"},
			{ID: "3", Type: "bugfix", Title: "fix 3"},
		},
	}

	engine := &ImpactEngine{
		store:  db,
		memory: mockMem,
	}

	res, err := engine.Analyze(ctx, "TestSym", "file.go", 3)
	if err != nil {
		t.Fatal(err)
	}

	// expected scores:
	// Blast radius: 1 caller -> bScore = log(2)/log(501) = 0.693147 / 6.216606 = 0.111499
	// cogScore: 50 / 100.0 = 0.5 (New)
	// churnScore: 0.8
	// testGap: 1.0
	// volumeScore: 100 / 500.0 = 0.2
	// runtimeScore: 0.5

	// expected base RiskScore:
	// (0.111499 * 0.20) = 0.0222998
	// (0.5 * 0.35)      = 0.175
	// (0.8 * 0.15)      = 0.12
	// (1.0 * 0.15)      = 0.15
	// (0.2 * 0.05)      = 0.01
	// (0.5 * 0.10)      = 0.05
	// Total: 0.0222998 + 0.175 + 0.12 + 0.15 + 0.01 + 0.05 = 0.5272998

	// expected final RiskScore with multiplier (3 bugfixes -> 1.6x)
	// 0.5272998 * 1.6 = 0.84367968

	if math.Abs(res.Target.RiskScore-0.8437) > 0.001 {
		t.Errorf("expected RiskScore ~0.8437, got %f", res.Target.RiskScore)
	}

	if !strings.Contains(res.Breakdown, "Historical Fragility (multiplier): 1.60x") {
		t.Errorf("expected Breakdown to contain fragility factor, got:\n%s", res.Breakdown)
	}
}

type mockSpecificImpactStore struct {
	*mockImpactStore
}

func (m *mockSpecificImpactStore) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]store.Symbol, error) {
	if name == "TestSym" {
		return []store.Symbol{
			{
				Name:       "TestSym",
				Path:       path,
				StartLine:  0,
				EndLine:    100,
				ChurnScore: 0.8,
				Pagerank:   0.5,
				Metrics: &types.SemanticMetrics{
					CognitiveComplexity: 50,
				},
			},
		}, nil
	}
	return nil, nil
}

func (m *mockSpecificImpactStore) GetCallersRecursive(ctx context.Context, symbol string, path string, maxDepth int) ([]store.Call, error) {
	if symbol == "TestSym" {
		return []store.Call{
			{CallerName: "Caller1", Path: path, Line: 1},
		}, nil
	}
	return nil, nil
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

func TestHealerImpactIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Initial State
	db := &mockSpecificImpactStore{
		mockImpactStore: &mockImpactStore{
			affectedTests: make(map[string][]store.Symbol),
		},
	}

	// Create an empty memory provider initially
	mockMem := &testMemoryProvider{}

	impact := NewImpactEngine(db, nil, mockMem)

	// Check initial risk
	resInitial, _ := impact.Analyze(ctx, "TestSym", "file.go", 3)
	initialRisk := resInitial.Target.RiskScore

	// 2. Simulate a Healer Fix
	// For this test we will just invoke recordInoculation manually or via a fake fix
	// But let's just use the logic directly
	healer := NewHealerEngine(nil, nil, nil, impact, nil, mockMem)

	target := &types.ASTPointer{
		Name:      "TestSym",
		Signature: "func()",
	}

	// Directly call the unexported method to simulate a successful fix
	healer.recordInoculation(ctx, target, "file.go", "test failure line 1")

	// Now memory provider should have 1 observation
	if len(mockMem.savedObservations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(mockMem.savedObservations))
	}

	// 3. Update the mock memory provider to return the saved observation in SearchInsights
	// so ImpactEngine can find it.
	mockMem.searchInsights = []types.MemoryInsight{
		{ID: "1", Type: mockMem.savedObservations[0].Type, Title: mockMem.savedObservations[0].Title},
	}

	// 4. Check Risk again
	resAfter, _ := impact.Analyze(ctx, "TestSym", "file.go", 3)
	afterRisk := resAfter.Target.RiskScore

	if afterRisk <= initialRisk {
		t.Errorf("expected risk score to increase after fix. Initial: %f, After: %f", initialRisk, afterRisk)
	}

	// It should increase by 1.2x
	expectedNewRisk := math.Min(1.0, initialRisk*1.2)
	if math.Abs(afterRisk-expectedNewRisk) > 0.001 {
		t.Errorf("expected new risk to be %f, got %f", expectedNewRisk, afterRisk)
	}
}
