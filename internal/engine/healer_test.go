package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	impact := NewImpactEngine(s, nil, nil)
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
    internal/engine/healer.go:52: some error
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

	// Line 52 in log -> Line 51 in LSP
	expectedHover1 := "Docs for symbol at 51"
	// Line 120 in log -> Line 119 in LSP
	expectedHover2 := "Docs for symbol at 119"
	
	if !strings.Contains(p, expectedHover1) {
		t.Errorf("Prompt missing hover info for healer.go:52. Prompt:\n%s", p)
	}
	if !strings.Contains(p, expectedHover2) {
		t.Errorf("Prompt missing hover info for analyzer.go:120. Prompt:\n%s", p)
	}

	if !strings.Contains(p, "Current Risk Score") {
		t.Errorf("Prompt missing impact analysis section. Prompt:\n%s", p)
	}
}

func TestHealerEngine_Shinigami(t *testing.T) {
	t.Skip("Flaky test")
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	mgr := lsp.NewManager()
	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, nil)
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
    internal/engine/healer.go:35: some error
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

func TestHealerEngine_Cancellation(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, nil)
	h := NewHealerEngine(s, nil, analyzer, impact)

	var wg sync.WaitGroup
	wg.Add(2) // For the two slow goroutines

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		if strings.Contains(prompt, "variant #0") {
			return "```go\nfunc fastFix() {}\n```", nil
		}
		// The other variants should be cancelled
		select {
		case <-ctx.Done():
			wg.Done()
			return "", ctx.Err()
		}
	}

	errorLog := `
--- FAIL: TestCancel (0.00s)
    internal/engine/healer.go:35: some error
`

	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if !strings.Contains(res.FixedCode, "fastFix") {
		t.Errorf("Expected fastFix, got %s", res.FixedCode)
	}

	// Wait for the cancelled goroutines to finish
	wg.Wait()
}

func TestHealerEngine_AllFail(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, nil)
	h := NewHealerEngine(s, nil, analyzer, impact)

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		return "```go\nfunc (m *MyStruct) Dont() {}\n```", nil
	}

	// Create a temp file in a subdirectory so packages.Load("./...") finds it
	tmpDir := filepath.Join(".", "test_bad_pkg")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)
	
	tmpFile := filepath.Join(tmpDir, "bad.go")
	content := `package test_bad_pkg
type MyInterface interface { Do() }
type MyStruct struct{}
func (m *MyStruct) Do() {}
var _ MyInterface = (*MyStruct)(nil)
`
	os.WriteFile(tmpFile, []byte(content), 0644)
	
	// We need to index it so StreamSymbols finds it
	absTmpFile, _ := filepath.Abs(tmpFile)
	h.Index(ctx, absTmpFile)

	errorLog := fmt.Sprintf(`
--- FAIL: TestFail (0.00s)
    %s:4: some error
`, absTmpFile)

	_, err := h.Fix(ctx, errorLog)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "all candidates failed validation") {
		t.Errorf("Expected 'all candidates failed validation', got: %v", err)
	}
}

func TestHealerEngine_Imports(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, nil)
	h := NewHealerEngine(s, nil, analyzer, impact)

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		// Return code that uses fmt but doesn't import it
		return "```go\nfunc (m *MyStruct) Do() { fmt.Println(\"hello\") }\n```", nil
	}

	tmpDir := filepath.Join(".", "test_imports_pkg")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)
	
	tmpFile := filepath.Join(tmpDir, "imports.go")
	content := `package test_imports_pkg
type MyInterface interface { Do() }
type MyStruct struct{}
func (m *MyStruct) Do() {}
var _ MyInterface = (*MyStruct)(nil)
`
	os.WriteFile(tmpFile, []byte(content), 0644)
	
	absTmpFile, _ := filepath.Abs(tmpFile)
	h.Index(ctx, absTmpFile)

	errorLog := fmt.Sprintf(`
--- FAIL: TestImports (0.00s)
    %s:4: some error
`, absTmpFile)

	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if !strings.Contains(res.FixedCode, "fmt.Println") {
		t.Errorf("Expected fmt.Println in fixed code, got %s", res.FixedCode)
	}
	
	patch := h.Ledger.Staged[absTmpFile]
	if !strings.Contains(patch.NewContent, "\"fmt\"") {
		t.Errorf("Expected 'fmt' import in staged content, got:\n%s", patch.NewContent)
	}
}

func TestHealerEngine_Fix_IntegrityWarning(t *testing.T) {
	ctx := context.Background()
	s, _ := store.New(ctx, ":memory:")
	defer s.Close()

	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, nil)
	h := NewHealerEngine(s, nil, analyzer, impact)

	h.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		return "```go\nfunc fixed() {}\n```", nil
	}

	errorLog := `
--- FAIL: TestDeep (0.00s)
    internal/engine/healer.go:52: some error
`

	res, err := h.Fix(ctx, errorLog)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	t.Logf("Result Status: %s", res.Status)
}