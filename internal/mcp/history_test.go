package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServer_SessionHistory(t *testing.T) {
	st, _ := store.NewStore(context.Background(), ":memory:")
	defer st.Close()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	server := setupMockServer(st, logger)

	// Test 1: Appending messages
	server.AppendSessionMessage(memory.Message{Role: "user", Content: "Hello"})
	server.AppendSessionMessage(memory.Message{Role: "assistant", Content: "Hi there"})

	history := server.GetTranscript(&mcp.CallToolRequest{})
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Content != "Hello" || history[1].Content != "Hi there" {
		t.Errorf("history content mismatch: %+v", history)
	}

	// Test 2: Bounded capacity (Ring Buffer)
	// MaxSessionHistory is 100
	for i := 0; i < 150; i++ {
		server.AppendSessionMessage(memory.Message{Role: "user", Content: "spam"})
	}

	history = server.GetTranscript(&mcp.CallToolRequest{})
	if len(history) != MaxSessionHistory {
		t.Fatalf("expected capped history at %d, got %d", MaxSessionHistory, len(history))
	}

	// Test 3: Concurrency (smoke test)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				server.AppendSessionMessage(memory.Message{Role: "user", Content: "concurrent"})
				server.GetTranscript(&mcp.CallToolRequest{})
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
