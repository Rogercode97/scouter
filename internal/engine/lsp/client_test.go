package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestJSONRPCClient(t *testing.T) {
	// We need a mock server binary or a way to simulate a process.
	// For testing, we can use a pipe and a goroutine that acts as a server.

	pr, pw := io.Pipe() // client read, server write
	sr, sw := io.Pipe() // server read, client write

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Mock server logic
	go func() {
		reader := bufio.NewReader(sr)
		for {
			var contentLength int
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				}
			}

			if contentLength == 0 {
				continue
			}

			body := make([]byte, contentLength)
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}

			// Basic JSON-RPC parsing
			var req JSONRPCRequest
			if err := json.Unmarshal(body, &req); err != nil {
				return
			}

			// Respond based on method
			var resp JSONRPCResponse
			resp.JSONRPC = "2.0"
			resp.ID = req.ID

			switch req.Method {
			case "initialize":
				resp.Result = json.RawMessage(`{"capabilities": {}}`)
			case "textDocument/definition":
				resp.Result = json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 5}}}]`)
			case "shutdown":
				resp.Result = json.RawMessage(`null`)
			}

			data, _ := json.Marshal(resp)
			fmt.Fprintf(pw, "Content-Length: %d\r\n\r\n%s", len(data), data)
		}
	}()

	client := &jsonrpcClient{
		reader: pr,
		writer: sw,
		done:   make(chan struct{}),
	}
	go client.listen()

	// The client expects an initialize call if we use NewClient,
	// but here we are using it manually.
	// Let's test the methods directly.

	// Test Definition
	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
			Position:     Position{Line: 1, Character: 1},
		},
	}

	// We need to implement the client logic to read/write headers as well if we follow LSP strictly.
	// But the prompt says "Basic JSON-RPC over stdio client".
	// LSP uses "Content-Length: ...\r\n\r\n{json}".

	locs, err := client.Definition(ctx, params)
	if err != nil {
		t.Fatalf("Definition failed: %v", err)
	}

	if len(locs) != 1 || locs[0].URI != "file:///test.go" {
		t.Errorf("Unexpected result: %v", locs)
	}
}
