package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestACCPStateTransitionIntegration(t *testing.T) {
	// Setup many symbols to exceed HOT threshold (default 500 tokens for this test)
	symbols := make([]store.Symbol, 100)
	for i := 0; i < 100; i++ {
		symbols[i] = store.Symbol{
			Name:      fmt.Sprintf("Func%d", i),
			Path:      "main.go",
			Type:      "function",
			StartLine: i,
			Doc:       "A very long documentation string to increase token count significantly. " + strings.Repeat("more tokens ", 20),
		}
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

	t.Run("handleSearch transitions to WARM when token count increases", func(t *testing.T) {
		req := &mcp.CallToolRequest{}
		args := SearchParams{
			Query:  "Func",
			Format: "hakai",
		}

		res, _, err := s.handleSearch(context.Background(), req, args)
		if err != nil {
			t.Fatalf("handleSearch failed: %v", err)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "|WARM") && !strings.Contains(text, "|COLD") {
			t.Errorf("expected state transition to WARM or COLD in output, but got always HOT. Output preview: %s", text[:100])
		}
	})

	t.Run("handleCallers transitions to WARM when token count increases", func(t *testing.T) {
		calls := make([]store.Call, 500)
		for i := 0; i < 500; i++ {
			calls[i] = store.Call{
				CallerName: fmt.Sprintf("Caller%d", i),
				Path:       "main.go",
			}
		}
		st.calls = calls

		req := &mcp.CallToolRequest{}
		args := CallersParams{
			CalleeName: "TestFunc",
			Format:     "hakai",
		}

		res, _, err := s.handleCallers(context.Background(), req, args)
		if err != nil {
			t.Fatalf("handleCallers failed: %v", err)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "|WARM") && !strings.Contains(text, "|COLD") {
			t.Errorf("expected state transition to WARM or COLD in output, but got always HOT. Output preview: %s", text[:100])
		}
	})
}
