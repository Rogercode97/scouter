package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestStreamWithTreeSitter_Calls(t *testing.T) {
	// Create a sample TS file
	content := []byte(`
		function caller() {
			callee();
			obj.method();
		}
		function callee() {}
	`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(filePath, content, 0644)

	ctx := t.Context()
	pointersIt, callsIt, err := StreamWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("StreamWithTreeSitter failed: %v", err)
	}

	var pointers []types.ASTPointer
	for p := range pointersIt {
		pointers = append(pointers, p)
	}

	var calls []types.ASTCall
	for c := range callsIt {
		calls = append(calls, c)
	}

	if len(calls) == 0 {
		t.Errorf("Expected calls, got 0")
	}

	foundCallee := false
	for _, call := range calls {
		if call.CalleeName == "callee" || call.CalleeName == "method" {
			foundCallee = true
		}
	}
	if !foundCallee {
		t.Errorf("Did not find expected calls")
	}

	if len(pointers) == 0 {
		t.Errorf("Expected pointers, got 0")
	}
}

func TestStreamWithTreeSitter_Doc(t *testing.T) {
	t.Run("TypeScript", func(t *testing.T) {
		content := []byte(`
			/**
			 * Greeter class
			 */
			class Greeter {
				// sayHello method
				sayHello() {}
			}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointersIt, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var pointers []types.ASTPointer
		for p := range pointersIt {
			pointers = append(pointers, p)
		}

		foundClass := false
		foundMethod := false
		for _, p := range pointers {
			if p.Name == "Greeter" {
				foundClass = true
				if p.Doc != "Greeter class" {
					t.Errorf("expected class doc 'Greeter class', got %q", p.Doc)
				}
			}
			if p.Name == "sayHello" {
				foundMethod = true
				if p.Doc != "sayHello method" {
					t.Errorf("expected method doc 'sayHello method', got %q", p.Doc)
				}
			}
		}
		if !foundClass || !foundMethod {
			t.Errorf("did not find expected symbols")
		}
	})

	t.Run("Python", func(t *testing.T) {
		content := []byte(`
def hello():
    """
    Python docstring
    """
    pass

class World:
    '''
    Python class docstring
    '''
    pass
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.py")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointersIt, _, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		var pointers []types.ASTPointer
		for p := range pointersIt {
			pointers = append(pointers, p)
		}

		foundFunc := false
		foundClass := false
		for _, p := range pointers {
			if p.Name == "hello" {
				foundFunc = true
				if p.Doc != "Python docstring" {
					t.Errorf("expected func doc 'Python docstring', got %q", p.Doc)
				}
			}
			if p.Name == "World" {
				foundClass = true
				if p.Doc != "Python class docstring" {
					t.Errorf("expected class doc 'Python class docstring', got %q", p.Doc)
				}
			}
		}
		if !foundFunc || !foundClass {
			t.Errorf("did not find expected symbols")
		}
	})
}

func TestStreamWithTreeSitter_RustImpl(t *testing.T) {
	content := []byte(`
		trait Animal {
			fn speak(&self);
		}

		struct Dog {}

		impl Animal for Dog {
			fn speak(&self) {
				println!("Woof");
			}
		}
	`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.rs")
	os.WriteFile(filePath, content, 0644)

	ctx := t.Context()
	pointersIt, callsIt, err := StreamWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("StreamWithTreeSitter failed: %v", err)
	}

	var pointers []types.ASTPointer
	for p := range pointersIt {
		pointers = append(pointers, p)
	}

	var calls []types.ASTCall
	for c := range callsIt {
		calls = append(calls, c)
	}

	foundImpl := false
	for _, call := range calls {
		if call.LinkType == "implements" && call.CallerName == "Dog" && call.CalleeName == "Animal" {
			foundImpl = true
		}
	}

	if !foundImpl {
		t.Errorf("Expected implements link from Dog to Animal, but did not find it. Calls found: %v", calls)
	}
}

