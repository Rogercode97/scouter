package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestStreamWithTreeSitter_Hierarchical(t *testing.T) {
	t.Run("TypeScript_Nested", func(t *testing.T) {
		content := []byte(`
			function A() {
				function B() {
					const c = () => {};
				}
			}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointersIt, _, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var pointers []types.ASTPointer
		for p := range pointersIt {
			pointers = append(pointers, p)
		}

		foundB := false
		foundC := false
		for _, p := range pointers {
			if p.Name == "A.B" {
				foundB = true
			}
			if p.Name == "A.B.c" {
				foundC = true
			}
		}
		if !foundB {
			t.Errorf("did not find A.B, got: %v", pointers)
		}
		if !foundC {
			t.Errorf("did not find A.B.c, got: %v", pointers)
		}
	})

	t.Run("Python_Lambda", func(t *testing.T) {
		content := []byte(`
def outer():
    x = lambda: print("hello")
`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.py")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointersIt, _, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var pointers []types.ASTPointer
		for p := range pointersIt {
			pointers = append(pointers, p)
		}

		foundLambda := false
		for _, p := range pointers {
			if p.Name == "outer.func1" {
				foundLambda = true
			}
		}
		if !foundLambda {
			t.Errorf("did not find outer.func1 (lambda), got: %v", pointers)
		}
	})

	t.Run("Rust_Closure", func(t *testing.T) {
		content := []byte(`
fn main() {
    let closure = |x: i32| x + 1;
}
`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.rs")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointersIt, _, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var pointers []types.ASTPointer
		for p := range pointersIt {
			pointers = append(pointers, p)
		}

		foundClosure := false
		for _, p := range pointers {
			if p.Name == "main.func1" {
				foundClosure = true
			}
		}
		if !foundClosure {
			t.Errorf("did not find main.func1 (closure), got: %v", pointers)
		}
	})

	t.Run("TypeScript_Call_Hierarchical", func(t *testing.T) {
		content := []byte(`
			function A() {
				function B() {
					callee();
				}
			}
			function callee() {}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		_, callsIt, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var calls []types.ASTCall
		for c := range callsIt {
			calls = append(calls, c)
		}

		foundCall := false
		for _, c := range calls {
			if c.CallerName == "A.B" && c.CalleeName == "callee" {
				foundCall = true
			}
		}
		if !foundCall {
			t.Errorf("did not find call from A.B to callee, got: %v", calls)
		}
	})
}
