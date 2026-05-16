package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

type healerMockLSPClient struct {
	lsp.LSPClient
	hoverFn func(ctx context.Context, params lsp.HoverParams) (*lsp.Hover, error)
}

func (m *healerMockLSPClient) Hover(ctx context.Context, params lsp.HoverParams) (*lsp.Hover, error) {
	if m.hoverFn != nil {
		return m.hoverFn(ctx, params)
	}
	return &lsp.Hover{Contents: lsp.MarkupContent{Kind: "plaintext", Value: "mocked hover"}}, nil
}

func (m *healerMockLSPClient) Close() error { return nil }

func TestHealerEngine_Fix_DeepRCA(t *testing.T) {
	ctx := context.Background()
	// 1. Setup Store, LSP, Analyzer, and Impact
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()
	
	mgr := lsp.NewManager()
	
	// Override client creator to return our mock
	mgr.SetClientCreatorForTest(func(ctx context.Context, dir, binary string, args ...string) (lsp.LSPClient, error) {
		return &healerMockLSPClient{
			hoverFn: func(ctx context.Context, params lsp.HoverParams) (*lsp.Hover, error) {
				return &lsp.Hover{
					Contents: lsp.MarkupContent{
						Kind:  "markdown",
						Value: fmt.Sprintf("Docs for symbol at %d", params.Position.Line),
					},
				}, nil
			},
		}, nil
	})

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil)
	h := NewHealerEngine(s, mgr, analyzer, impact)
	
	// Mock the LLM request
	var (
		mu             sync.RWMutex
		capturedPrompt string
	)
	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		capturedPrompt = prompt
		mu.Unlock()
		return "```go\nfunc fixed() {}\n```", nil
	}

	// 2. Mock error log with multiple frames
	errorLog := `
--- FAIL: TestDeep (0.00s)
    internal/engine/healer.go:50: some error
    internal/engine/analyzer.go:120: upstream failure
`

	// 3. Run Fix
	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if res == nil {
		t.Fatal("HealResult is nil")
	}

	// 4. Verify Enriched Prompt
	mu.RLock()
	p := capturedPrompt
	mu.RUnlock()

	// Line 50 in log -> Line 49 in LSP
	expectedHover1 := "Docs for symbol at 49"
	// Line 120 in log -> Line 119 in LSP
	expectedHover2 := "Docs for symbol at 119"
	
	if !strings.Contains(p, expectedHover1) {
		t.Errorf("Prompt missing hover info for healer.go:50. Prompt:\n%s", p)
	}
	if !strings.Contains(p, expectedHover2) {
		t.Errorf("Prompt missing hover info for analyzer.go:120. Prompt:\n%s", p)
	}

	if !strings.Contains(p, "Current Risk Score") {
		t.Errorf("Prompt missing impact analysis section. Prompt:\n%s", p)
	}
}

func TestHealerEngine_Shinigami(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	mgr := lsp.NewManager()
	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil)
	h := NewHealerEngine(s, mgr, analyzer, impact)

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		if strings.Contains(prompt, "variant #0") {
			return "```go\nfunc longFix() {\n" + strings.Repeat("// very long\n", 50) + "}\n```", nil
		}
		if strings.Contains(prompt, "variant #1") {
			return "```go\nfunc kissFix() {}\n```", nil
		}
		return "```go\nfunc otherFix() {}\n```", nil
	}

	errorLog := `
--- FAIL: TestShinigami (0.00s)
    internal/engine/healer.go:32: some error
`

	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Shinigami Fix failed: %v", err)
	}

	if res.Status != "STAGED" {
		t.Errorf("Expected status STAGED, got %s", res.Status)
	}

	if !strings.Contains(res.FixedCode, "kissFix") {
		t.Errorf("Verifier failed to select the KISS candidate. Got: %s", res.FixedCode)
	}

	if h.Ledger.Stats.FilesCount != 1 {
		t.Errorf("Expected 1 staged patch in ledger, got %d", h.Ledger.Stats.FilesCount)
	}
}

func TestHealerEngine_Fix_IntegrityWarning(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil)
	h := NewHealerEngine(s, nil, analyzer, impact)

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		return "```go\nfunc fixed() {}\n```", nil
	}

	errorLog := `
--- FAIL: TestDeep (0.00s)
    internal/engine/healer.go:50: some error
`

	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	t.Logf("Result Status: %s", res.Status)
}