package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

type mockLSPProvider struct {
	impls []lsp.Location
}

func (m *mockLSPProvider) GetClient(ctx context.Context, filePath string) (lsp.LSPClient, error) {
	return &mockLSPClient{impls: m.impls}, nil
}

type mockLSPClient struct {
	lsp.LSPClient
	impls []lsp.Location
}

func (m *mockLSPClient) Implementation(ctx context.Context, params lsp.ImplementationParams) ([]lsp.Location, error) {
	return m.impls, nil
}
func (m *mockLSPClient) Close() error { return nil }

func TestIntegration_InterfaceTracing(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_integration_interface.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Prepare Fixture Path
	absPath, _ := filepath.Abs("tests/fixtures/interface_sample.go")
	
	// 2. Index the file (Simplified index logic)
	err = s.SaveFileIndex(ctx, &store.FileIndex{
		Path: absPath,
		Hash: "samplehash",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// Manual symbol injection based on interface_sample.go
	// In a real run, engine.ParseFile would do this.
	symbols := []store.Symbol{
		{Name: "Shape", Type: "interface", Path: absPath, StartLine: 6, StartCol: 6, EndLine: 9},
		{Name: "Area", Type: "method_spec", Path: absPath, StartLine: 7, StartCol: 2, EndLine: 7}, // Interface method
		{Name: "Circle", Type: "struct", Path: absPath, StartLine: 11, StartCol: 6, EndLine: 13},
		{Name: "Area", Type: "method", Path: absPath, StartLine: 15, StartCol: 17, EndLine: 17},   // Circle implementation
	}
	for _, sym := range symbols {
		s.SaveSymbol(ctx, &sym)
	}

	// 3. Mock LSP to link Circle.Area back to Shape.Area
	mockProvider := &mockLSPProvider{
		impls: []lsp.Location{
			{
				URI: "file://" + absPath,
				Range: lsp.Range{
					Start: lsp.Position{Line: 6, Character: 1}, // Line 7 (0-based) is Area()
					End:   lsp.Position{Line: 6, Character: 10},
				},
			},
		},
	}

	// 4. Run Enricher
	en := engine.NewEnricher(s, mockProvider)
	if err := en.Enrich(ctx); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// 5. Verify: Get Impact of Circle.Area
	// It should find Shape.Area (distance 1)
	impactEngine := engine.NewImpactEngine(s, nil, nil)
	impact, err := impactEngine.Analyze(ctx, "Area", absPath, 3)
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}

	foundIface := false
	for _, res := range impact.Callers {
		if res.Symbol == "Area" && res.LinkType == "dynamic" {
			foundIface = true
			break
		}
	}

	if !foundIface {
		t.Errorf("Interface method not found in impact analysis: %+v", impact)
	}
}
