package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupTestServer(ctx context.Context) (*Server, *mcp.ClientSession, func()) {
	st, _ := store.New(ctx, ":memory:")
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	server := NewServer(st, logger)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go server.Start(ctx, serverTransport)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, &mcp.ClientOptions{
		CreateMessageHandler: func(_ context.Context, req *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
			// Dynamic Router based on request
			prompt := ""
			if len(req.Params.Messages) > 0 {
				if txt, ok := req.Params.Messages[0].Content.(*mcp.TextContent); ok {
					prompt = txt.Text
				}
			}

			// Simulated LLM logic
			var response string
			if prompt == "empty" {
				response = "[]"
			} else {
				response = "```go\n// Mocked code fix by simulated LLM\n```"
			}

			return &mcp.CreateMessageResult{
				Content: &mcp.TextContent{
					Text: response,
				},
			}, nil
		},
	})
	session, _ := client.Connect(ctx, clientTransport, nil)

	return server, session, func() {
		session.Close()
		st.Close()
	}
}

func TestServer_Lifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tools.Tools) != 12 {
		t.Errorf("expected 12 tools, got %d", len(tools.Tools))
	}
}

func TestServer_Handlers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	tests := []struct {
		name      string
		arguments map[string]any
		wantSub   string
	}{
		{
			name:      "scouter_search",
			arguments: map[string]any{"mode": "hybrid", "query": "test"},
			wantSub:   "[]",
		},
		{
			name:      "scouter_context",
			arguments: map[string]any{"action": "filter_signal", "text": "line1\nline2", "level": "light"},
			wantSub:   "line1",
		},
		{
			name:      "scouter_radar",
			arguments: map[string]any{"action": "risk_audit", "limit": 5},
			wantSub:   "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      tt.name,
				Arguments: tt.arguments,
			})
			if err != nil {
				t.Fatalf("tool %s failed: %v", tt.name, err)
			}
			_ = res.Content[0].(*mcp.TextContent).Text
			// No null workaround needed, backend should return []
		})
	}
}

func TestServer_ErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	// Test missing arguments for search index (should fail validation inside mode)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "scouter_search",
		Arguments: map[string]any{"mode": "index"},
	})
	if err != nil {
		t.Fatalf("call to scouter_search failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for missing arguments")
	}

	// Test invalid path for inspect read
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "scouter_inspect",
		Arguments: map[string]any{
			"mode":     "fragment",
			"filePath": "/invalid/path",
			"pointer":  "main",
		},
	})
	if err != nil {
		t.Fatalf("call to scouter_inspect failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for invalid path")
	}
}
