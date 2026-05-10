package engine

import (
	"os"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestStructuralSearch(t *testing.T) {
	content := `package main
func main() {
	println("hello")
	println("world")
}
func foo() {
	println("bar")
}`
	tmpFile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	os.WriteFile(tmpFile.Name(), []byte(content), 0644)

	ctx := t.Context()
	
	t.Run("match function declaration", func(t *testing.T) {
		// Use a valid identifier as wildcard for now
		matches, err := StructuralSearch(ctx, tmpFile.Name(), "println($X)", ".go")
		if err != nil {
			t.Fatal(err)
		}
		// In current implementation, "X" must match "X". 
		// Let's modify matchNodes to treat uppercase identifiers as wildcards.
		if len(matches) != 3 {
			t.Errorf("got %d matches, want 3", len(matches))
		}
	})

	t.Run("match and capture variable", func(t *testing.T) {
		matches, err := StructuralSearch(ctx, tmpFile.Name(), "println($X)", ".go")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Fatal("expected matches, got 0")
		}
		
		// The current implementation might match, but it doesn't return captures.
		// We expect the first match to have $X = "hello" (including quotes in Go)
		m := matches[0]
		val, ok := m.Captures["$X"]
		if !ok {
			t.Errorf("expected capture $X, not found")
		}
		if val != "\"hello\"" {
			t.Errorf("got capture $X = %s, want \"hello\"", val)
		}
	})

	t.Run("match multiple nodes with $$$ in middle", func(t *testing.T) {
		content := `package main
func main() {
	A()
	B()
	C()
	D()
}`
		tmpFile, err := os.CreateTemp("", "test2*.go")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte(content), 0644)

		// $$$ should match B() and C()
		matches, err := StructuralSearch(ctx, tmpFile.Name(), "func main() { A(); $$$; D() }", ".go")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Fatal("expected matches, got 0")
		}

		m := matches[0]
		val, ok := m.Captures["$$$"]
		if !ok {
			t.Errorf("expected capture $$$, not found")
		}
		// It should capture B() and C()
		// Tree-sitter might include the semicolons or newlines
		if !strings.Contains(val, "B()") || !strings.Contains(val, "C()") {
			t.Errorf("got capture $$$ = %q, want it to contain B() and C()", val)
		}
	})

	t.Run("match with inside relational rule", func(t *testing.T) {
		content := `package main
func foo() {
	return nil
}
var x = func() { return nil }
`
		tmpFile, err := os.CreateTemp("", "test_inside*.go")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte(content), 0644)

		rule := types.StructuralRule{
			Pattern: "return nil",
			Inside:  "function_declaration",
		}
		matches, err := StructuralSearchWithRule(ctx, tmpFile.Name(), rule, ".go")
		if err != nil {
			t.Fatal(err)
		}
		// Should only match the one inside foo(), not the one inside the func literal (which is a func_literal, not function_declaration)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].StartLine != 3 {
			t.Errorf("expected match on line 3, got %d", matches[0].StartLine)
		}
	})

	t.Run("match with has relational rule", func(t *testing.T) {
		content := `package main
func foo() {
	println("hello")
}
func bar() {
	return nil
}
`
		tmpFile, err := os.CreateTemp("", "test_has*.go")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte(content), 0644)

		rule := types.StructuralRule{
			Pattern: "func $NAME() { $$$ }",
			Has:     "return nil",
		}
		matches, err := StructuralSearchWithRule(ctx, tmpFile.Name(), rule, ".go")
		if err != nil {
			t.Fatal(err)
		}
		// Should only match bar()
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		val, ok := matches[0].Captures["$NAME"]
		if !ok || val != "bar" {
			t.Errorf("expected capture $NAME = bar, got %v", val)
		}
	})

	t.Run("structural refactor with interpolation", func(t *testing.T) {
		content := `package main
func main() {
	println("hello")
	println("world")
}`
		tmpFile, err := os.CreateTemp("", "test_refactor*.go")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte(content), 0644)

		pattern := "println($X)"
		template := "log.Printf(\"DEBUG: %v\", $X)"
		
		newContent, err := StructuralRefactor(ctx, tmpFile.Name(), pattern, template, ".go")
		if err != nil {
			t.Fatal(err)
		}

		expected1 := "log.Printf(\"DEBUG: %v\", \"hello\")"
		expected2 := "log.Printf(\"DEBUG: %v\", \"world\")"
		
		if !strings.Contains(newContent, expected1) || !strings.Contains(newContent, expected2) {
			t.Errorf("refactor failed.\nGot:\n%s\nWant it to contain %q and %q", newContent, expected1, expected2)
		}
	})
}
