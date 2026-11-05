package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func TestOmniscienceCallHierarchy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cwd, _ := os.Getwd()
	// If running from tests/ folder, go up one level
	projectRoot := cwd
	if filepath.Base(cwd) == "tests" {
		projectRoot = filepath.Dir(cwd)
	}

	dbPath := filepath.Join(projectRoot, "scouter_test.db")
	defer os.Remove(dbPath)

	st, err := store.NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// 1. Index a known file to populate the store
	targetFile := filepath.Join(projectRoot, "internal/engine/ripple.go")
	absPath, _ := filepath.Abs(targetFile)

	err = st.SaveFileIndex(ctx, &store.FileIndex{
		Path:    absPath,
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("failed to save file index: %v", err)
	}

	// Manual symbol injection for the test
	//
	// The position is read from the file instead of being hard-coded: a fixed
	// line number breaks on every edit above the declaration, and the failure
	// surfaces as "column is beyond end of line" from the language server,
	// which reads like an LSP defect rather than a stale constant.
	declLine, err := findDeclLine(absPath, "func NewRippleEngine")
	if err != nil {
		t.Fatalf("locate NewRippleEngine: %v", err)
	}

	err = st.SaveSymbol(ctx, &store.Symbol{
		Name:      "NewRippleEngine",
		Path:      absPath,
		StartLine: declLine,
		StartCol:  5,
		Type:      "function",
	})
	if err != nil {
		t.Fatalf("failed to save symbol: %v", err)
	}

	// 2. Initialize LSP Manager
	// Skip if gopls is not found in PATH or ~/go/bin/gopls
	if _, err := exec.LookPath("gopls"); err != nil {
		home, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(home, "go", "bin", "gopls")); err != nil {
			t.Skip("gopls not found, skipping TestOmniscienceCallHierarchy")
		}
	}

	mgr := lsp.GetGlobalManager()

	client, err := mgr.GetClient(ctx, absPath)
	if err != nil {
		t.Fatalf("failed to get lsp client: %v", err)
	}

	// 3. Prepare Call Hierarchy
	t.Logf("Preparing Call Hierarchy for NewRippleEngine at %s:%d", absPath, declLine)
	items, err := client.PrepareCallHierarchy(ctx, lsp.CallHierarchyPrepareParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + absPath},
			Position: lsp.Position{
				Line:      declLine - 1, // the protocol counts from zero
				Character: 5,
			},
		},
	})
	if err != nil {
		t.Fatalf("PrepareCallHierarchy failed: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("no call hierarchy items found")
	}

	t.Logf("Found item: %s (%s)", items[0].Name, items[0].URI)

	// 4. Get Incoming Calls
	calls, err := client.IncomingCalls(ctx, lsp.CallHierarchyIncomingCallsParams{
		Item: items[0],
	})
	if err != nil {
		t.Fatalf("IncomingCalls failed: %v", err)
	}

	t.Logf("Found %d incoming calls", len(calls))
	for _, call := range calls {
		t.Logf("- Caller: %s from %s", call.From.Name, call.From.URI)
	}

	if len(calls) == 0 {
		t.Errorf("expected at least one caller for NewRippleEngine (e.g., from internal/mcp/server.go)")
	}
}

// findDeclLine returns the 1-based line holding needle, so a test that needs a
// symbol's position reads it from the source instead of hard-coding a number
// that the next edit above the declaration invalidates.
func findDeclLine(path, needle string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, needle) {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("%q not found in %s", needle, path)
}
