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
			return &mcp.CreateMessageResult{
				Content: &mcp.TextContent{
					Text: "```go\n// Mocked code fix by simulated LLM\n```",
				},
			}, nil
		},
	})
	session, _ := client.Connect(ctx, clientTransport, nil)

	// Unlock heavy arsenal for tests
	_, _ = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "unlock_heavy_arsenal",
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

	if len(tools.Tools) != 28 {
		t.Errorf("expected 28 tools, got %d", len(tools.Tools))
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
			name:      "search",
			arguments: map[string]any{"query": "test"},
			wantSub:   "[]",
		},
		{
			name:      "pure_signal",
			arguments: map[string]any{"text": "line1\nline2", "level": "light"},
			wantSub:   "line1",
		},
		{
			name:      "critical_code",
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
			textContent := res.Content[0].(*mcp.TextContent).Text
			if textContent == "null" {
				textContent = "[]" // SDK normalization for empty results in some cases
			}
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
		Name:      "index",
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
		Name: "read",
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
}
