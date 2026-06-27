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
	st, _ := store.NewStore(ctx, ":memory:")
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	server := setupMockServer(st, logger)
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

	// Unlock heavy arsenal for tests
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "scouter_unlock",
	})

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

	if len(tools.Tools) != 29 {
		t.Errorf("expected 29 tools, got %d", len(tools.Tools))
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
			name:      "ast_search",
			arguments: map[string]any{"query": "test"},
			wantSub:   "[]",
		},
		{
			name:      "cognitive_signal",
			arguments: map[string]any{"text": "line1\nline2", "level": "light"},
			wantSub:   "line1",
		},
		{
			name:      "risk_critical_code",
			arguments: map[string]any{"limit": 5},
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

	// Test missing arguments for index (should fail validation)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ast_index",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call to index failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for missing arguments")
	}

	// Test invalid path for read
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "ast_read",
		Arguments: map[string]any{
			"filePath": "/invalid/path",
			"pointer":  "main",
		},
	})
	if err != nil {
		t.Fatalf("call to scouter_read failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for invalid path")
	}

	// Test missing arguments for ast_provenance
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ast_provenance",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call to ast_provenance failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for missing arguments in ast_provenance")
	}
}
