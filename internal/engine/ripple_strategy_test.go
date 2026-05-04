package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

type rippleMockStore struct {
	store.Repository
	callers map[string][]store.Call
	callees map[string][]store.Call
	symbols map[string][]store.Symbol
}

func (m *rippleMockStore) GetCallers(ctx context.Context, callee string, limit, offset int) ([]store.Call, error) {
	return m.callers[callee], nil
}

func (m *rippleMockStore) GetCallees(ctx context.Context, caller string) ([]store.Call, error) {
	return m.callees[caller], nil
}

func (m *rippleMockStore) SearchSymbols(ctx context.Context, q, t string, limit, offset int) ([]store.Symbol, error) {
	return m.symbols[q], nil
}

type mockImpactEngine struct {
	callers map[string][]store.Call
}

func (m *mockImpactEngine) GetDeterministicCallers(ctx context.Context, symbolName string) ([]store.Call, error) {
	if calls, ok := m.callers[symbolName]; ok {
		return calls, nil
	}
	return nil, fmt.Errorf("not found")
}

func TestBFSPropagationStrategy_Discover(t *testing.T) {
	ms := &rippleMockStore{
		callers: map[string][]store.Call{
			"SymA": {
				{CallerName: "SymB", Path: "fileB.go"},
			},
			"SymB": {
				{CallerName: "SymC", Path: "fileC.go"},
			},
		},
		symbols: map[string][]store.Symbol{
			"SymA": {
				{Name: "SymA", Path: "fileA.go"},
			},
		},
	}

	mie := &ImpactEngine{
		store: ms,
		// LSPManager: nil, // Will cause GetDeterministicCallers to fail and fallback
	}

	strategy := &BFSPropagationStrategy{
		store:        ms,
		impactEngine: mie,
	}

	ctx := context.Background()
	found := make(map[string]bool)
	
	for task, err := range strategy.Discover(ctx, "SymA", 2) {
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}
		key := task.SymbolName + ":" + task.FilePath
		found[key] = true
	}

	expected := []string{
		"SymA:fileA.go", // Depth 0 (self)
		"SymB:fileB.go", // Depth 1
		"SymC:fileC.go", // Depth 2
	}

	for _, exp := range expected {
		if !found[exp] {
			t.Errorf("Expected to find %s, but didn't", exp)
		}
	}
}
