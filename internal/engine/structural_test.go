package engine

import (
	"os"
	"testing"
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

	t.Run("match specific call", func(t *testing.T) {
		matches, err := StructuralSearch(ctx, tmpFile.Name(), `println("hello")`, ".go")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Errorf("got %d matches, want 1", len(matches))
		}
	})
}
