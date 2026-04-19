package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseWithTreeSitter_Calls(t *testing.T) {
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

	ctx := context.Background()
	pointers, calls, err := ParseWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("ParseWithTreeSitter failed: %v", err)
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

func TestParseWithTreeSitter_Doc(t *testing.T) {
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

		ctx := context.Background()
		pointers, _, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
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

		ctx := context.Background()
		pointers, _, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
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
