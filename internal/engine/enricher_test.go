package engine

import (
	"context"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

type mockStore struct {
	store.Store
	methods        []store.Symbol
	symbolsByRange []store.Symbol
	savedCalls     []store.Call
}

func (m *mockStore) GetSymbolsByType(ctx context.Context, symType string) ([]store.Symbol, error) {
	return m.methods, nil
}

func (m *mockStore) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]store.Symbol, error) {
	return m.symbolsByRange, nil
}

func (m *mockStore) SaveCall(ctx context.Context, c store.Call) error {
	m.savedCalls = append(m.savedCalls, c)
	return nil
}

func (m *mockStore) WithTransaction(ctx context.Context, fn func(context.Context, store.Store) error) error {
	return fn(ctx, m)
}

type mockLSPClient struct {
	lsp.LSPClient
	impls []lsp.Location
}

func (m *mockLSPClient) Implementation(ctx context.Context, params lsp.ImplementationParams) ([]lsp.Location, error) {
	return m.impls, nil
}

func (m *mockLSPClient) Close() error { return nil }

type mockLSPProvider struct {
	client lsp.LSPClient
	err    error
}

func (m *mockLSPProvider) GetClient(ctx context.Context, filePath string) (lsp.LSPClient, error) {
	return m.client, m.err
}

func TestEnricher_Enrich(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	circlePath := cwd + "/circle_test.go"
	shapePath := cwd + "/shape_test.go"

	// Create dummy files to satisfy ValidatePath existence check
	os.WriteFile(circlePath, []byte("package test"), 0644)
	os.WriteFile(shapePath, []byte("package test"), 0644)
	defer os.Remove(circlePath)
	defer os.Remove(shapePath)

	mStore := &mockStore{
		methods: []store.Symbol{
			{Name: "Circle.Area", Path: circlePath, Type: "method", StartLine: 10, StartCol: 5},
		},
		symbolsByRange: []store.Symbol{
			{Name: "Shape:Area", Path: shapePath, Type: "method_spec", StartLine: 5, StartCol: 1},
		},
	}

	mLSP := &mockLSPClient{
		impls: []lsp.Location{
			{URI: "file://" + shapePath, Range: lsp.Range{Start: lsp.Position{Line: 4}}},
		},
	}

	mProvider := &mockLSPProvider{client: mLSP}

	enricher := NewEnricher(mStore, mProvider)
	err := enricher.Enrich(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mStore.savedCalls) != 1 {
		t.Fatalf("expected 1 saved call, got %d", len(mStore.savedCalls))
	}

	call := mStore.savedCalls[0]
	if call.CallerName != "Shape:Area" {
		t.Errorf("expected caller Shape:Area, got %s", call.CallerName)
	}
	if call.CalleeName != "Circle.Area" {
		t.Errorf("expected callee Circle.Area, got %s", call.CalleeName)
	}
	if call.LinkType != "dynamic" {
		t.Errorf("expected link type dynamic, got %s", call.LinkType)
	}
}
