package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

type mockStore struct {
	results []types.TestResult
}

func (m *mockStore) SaveTestResult(ctx context.Context, res *types.TestResult) error {
	m.results = append(m.results, *res)
	return nil
}

func TestHealthEngine_Ingest(t *testing.T) {
	jsonInput := `
	{"Time":"2024-04-19T02:00:00Z","Action":"run","Package":"github.com/org/repo","Test":"TestSaveSymbol"}
	{"Time":"2024-04-19T02:00:01Z","Action":"output","Package":"github.com/org/repo","Test":"TestSaveSymbol","Output":"some logs\n"}
	{"Time":"2024-04-19T02:00:02Z","Action":"output","Package":"github.com/org/repo","Test":"TestSaveSymbol","Output":"--- FAIL: TestSaveSymbol (0.12s)\n"}
	{"Time":"2024-04-19T02:00:02Z","Action":"output","Package":"github.com/org/repo","Test":"TestSaveSymbol","Output":"    store_test.go:45: expected nil error, got 'database locked'\n"}
	{"Time":"2024-04-19T02:00:03Z","Action":"fail","Package":"github.com/org/repo","Test":"TestSaveSymbol","Elapsed":0.12}
	`

	store := &mockStore{}
	engine := NewHealthEngine(store)

	err := engine.Ingest(context.Background(), strings.NewReader(jsonInput))
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if len(store.results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(store.results))
	}

	res := store.results[0]
	if res.TestName != "TestSaveSymbol" {
		t.Errorf("Expected TestSaveSymbol, got %s", res.TestName)
	}
	if res.Status != "fail" {
		t.Errorf("Expected fail status, got %s", res.Status)
	}
	if res.TargetSymbol != "SaveSymbol" {
		t.Errorf("Expected TargetSymbol SaveSymbol, got %s", res.TargetSymbol)
	}
	if !strings.Contains(res.ErrorMessage, "expected nil error") {
		t.Errorf("ErrorMessage missing content: %s", res.ErrorMessage)
	}
	if !strings.Contains(res.StackTrace, "store_test.go:45") {
		t.Errorf("StackTrace missing content: %s", res.StackTrace)
	}
}

func TestMapToSymbol(t *testing.T) {
	tests := []struct {
		testName string
		expected string
	}{
		{"TestSaveSymbol", "SaveSymbol"},
		{"TestSaveSymbol_ErrorHandling", "SaveSymbol"},
		{"TestStore_SaveSymbol", "Store.SaveSymbol"},
		{"TestSomethingRandom", "SomethingRandom"},
	}

	for _, tt := range tests {
		got := mapToSymbol(tt.testName)
		if got != tt.expected {
			t.Errorf("mapToSymbol(%s) = %s; want %s", tt.testName, got, tt.expected)
		}
	}
}
