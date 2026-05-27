package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockStore struct {
	store.Store
	symbols []store.Symbol
	calls   []store.Call
}

func (m *mockStore) SearchSymbols(ctx context.Context, query string, symType string, limit, offset int) ([]store.Symbol, error) {
	return m.symbols, nil
}

func (m *mockStore) GetCallers(ctx context.Context, calleeName string, limit, offset int) ([]store.Call, error) {
	return m.calls, nil
}

func (m *mockStore) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]store.Symbol, error) {
	var result []store.Symbol
	for _, sym := range m.symbols {
		if sym.Name == name && sym.Path == path {
			result = append(result, sym)
		}
	}
	return result, nil
}

func TestSovereignMCPIntegration(t *testing.T) {
	symbols := []store.Symbol{
		{Name: "TestFunc", Path: "main.go", Type: "function", StartLine: 10},
	}
	st := &mockStore{
		symbols: symbols,
	}
	searchEngine := engine.NewSearchEngine(st, nil)
	truthEngine := engine.NewTruthEngine(st, nil, nil, nil, nil, searchEngine, nil, nil, nil, nil, nil, nil, nil, nil)
	s := &Server{
		store:  st,
		engine: truthEngine,
	}

	t.Run("handleSearch returns Sovereign header", func(t *testing.T) {
		req := &mcp.CallToolRequest{}
		args := SearchArgs{
			Mode:   "text",
			Query:  "Test",
			Format: "hakai",
		}

		res, _, err := s.HandleSearch(context.Background(), req, args)
		if err != nil {
			t.Fatalf("handleSearch failed: %v", err)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, display.ProtocolHeader) {
			t.Errorf("expected output to contain %q, but it didn't. Output:\n%s", display.ProtocolHeader, text)
		}
		
		if !strings.Contains(text, "HOT") {
		    t.Errorf("expected output to contain state 'HOT', but it didn't. Output:\n%s", text)
		}
		})

		t.Run("handleCallers returns Sovereign header", func(t *testing.T) {
		req := &mcp.CallToolRequest{}
		calls := make([]store.Call, 21)
		for i := 0; i < 21; i++ {
			calls[i] = store.Call{CallerName: "Main", Path: "main.go"}
		}
		st := &mockStore{
			calls: calls,
		}
		s := &Server{
			store: st,
		}
		args := InspectArgs{
			Mode:   "callers",
			Symbol: "TestFunc",
		}

		res, _, err := s.HandleInspect(context.Background(), req, args)
		if err != nil {
			t.Fatalf("handleCallers failed: %v", err)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, display.ProtocolHeader) {
			t.Errorf("expected output to contain %q, but it didn't. Output:\n%s", display.ProtocolHeader, text)
		}
		})
		}