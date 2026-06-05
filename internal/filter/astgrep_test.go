package filter

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestDetectLang(t *testing.T) {
	a := &AstGrepFilter{}
	tests := []struct {
		file string
		want string
	}{
		{"main.go", "go"},
		{"app.js", "javascript"},
		{"script.ts", "typescript"},
		{"component.tsx", "tsx"},
		{"style.css", "css"},
		{"README.md", ""},
	}

	for _, tt := range tests {
		got := a.detectLang(map[string]any{"file": tt.file})
		if got != tt.want {
			t.Errorf("detectLang(%q) = %q, want %q", tt.file, got, tt.want)
		}
	}
}

func TestAstGrepApply(t *testing.T) {
	// Skip if sg is not installed
	if _, err := exec.LookPath("sg"); err != nil {
		t.Skip("ast-grep (sg) not found in path, skipping integration test")
	}

	ctx := context.Background()
	input := ActionResult{
		Lines: []string{
			"package main",
			"func main() {",
			"    println(\"hello\")",
			"}",
		},
		Metadata: map[string]any{"file": "main.go"},
	}

	t.Run("search", func(t *testing.T) {
		params := map[string]any{
			"pattern": "println($$$)",
		}
		res, err := astGrepAction(ctx, input, params)
		if err != nil {
			t.Fatalf("ast_grep failed: %v", err)
		}

		if len(res.Lines) == 0 {
			t.Errorf("expected matched lines, got 0")
		}

		found := false
		for _, line := range res.Lines {
			if strings.Contains(line, "println(\"hello\")") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected line 'println(\"hello\")' in output, got: %v", res.Lines)
		}
	})

	t.Run("rewrite", func(t *testing.T) {
		params := map[string]any{
			"pattern": "println($MSG)",
			"rewrite": "log.Info($MSG)",
		}
		res, err := astGrepAction(ctx, input, params)
		if err != nil {
			t.Fatalf("ast_grep rewrite failed: %v", err)
		}

		found := false
		for _, line := range res.Lines {
			if strings.Contains(line, "log.Info(\"hello\")") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected rewritten line 'log.Info(\"hello\")', got: %v", res.Lines)
		}
	})
}
