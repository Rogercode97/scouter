package scoutercmd

import (
	"context"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
)

func TestRunRejectsCd(t *testing.T) {
	ctx := context.Background()

	// Create dummy files for stdout/stderr
	fOut, _ := os.CreateTemp("", "out")
	defer os.Remove(fOut.Name())
	fErr, _ := os.CreateTemp("", "err")
	defer os.Remove(fErr.Name())

	code := Execute(ctx, []string{"cd", "/tmp"}, fOut, fErr)
	if code == 0 {
		t.Errorf("Execute(cd) should fail, got %d", code)
	}
}

func TestLeftoverMentions_FlagsHalfRenames(t *testing.T) {
	ledger := &engine.Ledger{Staged: map[string]engine.Patch{
		"rewritten.go":     {FilePath: "rewritten.go", Original: "foo()", NewContent: "bar()"},
		"half.go":          {FilePath: "half.go", Original: "func foo() {}\nfoo()", NewContent: "func bar() {}\nfoo()"},
		"untouched.go":     {FilePath: "untouched.go", Original: "// mentions foo\n", NewContent: "// mentions foo\n"},
		"unrelated.go":     {FilePath: "unrelated.go", Original: "a", NewContent: "b"},
	}}

	got := leftoverMentions(ledger, "foo")
	if len(got) != 1 || got[0] != "half.go" {
		t.Errorf("expected only half.go flagged, got %v", got)
	}
}

func TestParsePropagateTransformation(t *testing.T) {
	got, err := parsePropagateTransformation("rename:NewName")
	if err != nil {
		t.Fatalf("rename: form should parse, got: %v", err)
	}
	if got != "NewName" {
		t.Errorf("expected rewrite %q, got %q", "NewName", got)
	}

	for _, bad := range []string{"rename:", "NewName", "rewrite:NewName", ""} {
		if _, err := parsePropagateTransformation(bad); err == nil {
			t.Errorf("transformation %q should be rejected, got nil error", bad)
		}
	}
}

func TestIndexHelpExecution(t *testing.T) {
	ctx := context.Background()

	fOut, _ := os.CreateTemp("", "out")
	defer os.Remove(fOut.Name())
	fErr, _ := os.CreateTemp("", "err")
	defer os.Remove(fErr.Name())

	code := Execute(ctx, []string{"index", "--help"}, fOut, fErr)
	if code != 0 {
		t.Errorf("Execute(index --help) should succeed, got %d", code)
	}
}

func TestContextHelpExecution(t *testing.T) {
	ctx := context.Background()

	fOut, _ := os.CreateTemp("", "out")
	defer os.Remove(fOut.Name())
	fErr, _ := os.CreateTemp("", "err")
	defer os.Remove(fErr.Name())

	code := Execute(ctx, []string{"context", "--help"}, fOut, fErr)
	if code != 0 {
		t.Errorf("Execute(context --help) should succeed, got %d", code)
	}
}

