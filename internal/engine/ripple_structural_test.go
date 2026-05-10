package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestRippleEngine_StructuralIntegration(t *testing.T) {
	// 1. Setup temporary files
	tmpDir, err := os.MkdirTemp("", "ripple_structural_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fileA := filepath.Join(tmpDir, "fileA.go")
	contentA := `package main
func main() {
	foo("hello")
}`
	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatal(err)
	}

	fileB := filepath.Join(tmpDir, "fileB.go")
	contentB := `package main
func foo(s string) {
	println(s)
}`
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Setup mock store
	ms := &rippleEngineMockStore{
		callers: map[string][]store.Call{
			"foo": {{CallerName: "main", Path: fileA}},
		},
		symbols: map[string][]store.Symbol{
			"foo": {{Name: "foo", Path: fileB}},
		},
	}
	ie := &ImpactEngine{store: ms}

	// 3. Initialize StructuralTransformer
	// We want to change foo($ARG) to bar($ARG, true)
	st := &StructuralTransformer{
		Pattern: "$SYMBOL($ARG)",
	}

	engine := NewRippleEngine(ms, st, ie)
	ctx := context.Background()

	// 4. Propagate
	// Transformation template: bar($ARG, true)
	transformation := "bar($ARG, true)"
	ledger, err := engine.Propagate(ctx, "foo", transformation, 1)
	if err != nil {
		t.Fatalf("Propagate failed: %v", err)
	}

	if len(ledger.Staged) != 2 {
		t.Errorf("Expected 2 staged files, got %d", len(ledger.Staged))
	}

	// 5. Verify results
	newA := ledger.Staged[fileA].NewContent
	if !strings.Contains(newA, `bar("hello", true)`) {
		t.Errorf("fileA not correctly transformed.\nGot:\n%s", newA)
	}

	newB := ledger.Staged[fileB].NewContent
	if newB == "" {
		t.Errorf("fileB was not staged or content is empty")
	}
	// We don't necessarily expect fileB to change with the current pattern foo($ARG)
	// because it's a definition 'func foo(s string)'.
}
