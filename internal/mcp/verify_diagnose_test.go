package mcp

import (
	"context"
	"fmt"
	"testing"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/Rogercode97/scouter/internal/store"
	"log/slog"
	"os"
)

func TestVerifyDiagnose(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	st, _ := store.New(context.Background(), ":memory:")
	server := NewServer(st, logger)

	errorLog := `--- FAIL: TestDummy (0.00s)
    internal/engine/healer_test.go:15: dummy error
FAIL`

	req := &mcp.CallToolRequest{}
	args := DiagnoseParams{
		ErrorLog: errorLog,
	}

	res, _, err := server.handleDiagnose(context.Background(), req, args)
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	for _, content := range res.Content {
		if txt, ok := content.(*mcp.TextContent); ok {
			fmt.Println(txt.Text)
		}
	}
}
