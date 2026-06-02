package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestStreamWithTreeSitter_Anonymous(t *testing.T) {
	t.Run("Go", func(t *testing.T) {
		content := []byte(`
			package main
			func main() {
				go func() {
					println("hello")
				}()
				f := func(x int) {
					println(x)
				}
				f(1)
			}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.go")
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

		foundAnon := 0
		for _, p := range pointers {
			if strings.Contains(p.Name, ".func") {
				foundAnon++
			}
		}
		// Expecting at least 2: the goroutine closure and the variable assignment closure
		if foundAnon < 2 {
			t.Errorf("expected at least 2 anonymous functions, got %d", foundAnon)
		}
	})

	t.Run("TypeScript", func(t *testing.T) {
		content := []byte(`
			const anon = () => { console.log("hello") };
			[1, 2].forEach(x => console.log(x));
			function named() {
				return function() { return 42 };
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

		foundAnon := 0
		for _, p := range pointers {
			if strings.Contains(p.Name, ".func") {
				foundAnon++
			}
		}
		// Expecting 3: arrow function 'anon', arrow function in 'forEach', and function expression in 'named'
		if foundAnon < 3 {
			t.Errorf("expected at least 3 anonymous functions, got %d", foundAnon)
		}
	})
}

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
	pointersIt, callsIt, _, err := StreamWithTreeSitter(ctx, filePath)
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
		pointersIt, _, _, err := StreamWithTreeSitter(ctx, filePath)
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
			if p.Name == "Greeter.sayHello" {
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
		pointersIt, _, _, err := StreamWithTreeSitter(ctx, filePath)
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
	pointersIt, callsIt, _, err := StreamWithTreeSitter(ctx, filePath)
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

func TestStreamWithTreeSitter_PythonMethods(t *testing.T) {
	content := []byte(`
class Calculator:
    def add(self, a, b):
        return a + b
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

	foundMethod := false
	for _, p := range pointers {
		if p.Name == "Calculator.add" && p.Type == "method" {
			foundMethod = true
		}
	}

	if !foundMethod {
		t.Errorf("Expected method 'Calculator.add', got: %v", pointers)
	}
}

func TestStreamWithTreeSitter_RustMethods(t *testing.T) {
	content := []byte(`
struct Calculator;
impl Calculator {
    fn add(&self, a: i32, b: i32) -> i32 {
        a + b
    }
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

	foundMethod := false
	for _, p := range pointers {
		if p.Name == "Calculator.add" && p.Type == "method" {
			foundMethod = true
		}
	}

	if !foundMethod {
		t.Errorf("Expected method 'Calculator.add', got: %v", pointers)
	}
}

func TestStreamWithTreeSitter_DataFlow(t *testing.T) {
	content := []byte(`
		const x = 10;
		let y = x;
		y = 20;
	`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(filePath, content, 0644)

	ctx := t.Context()
	_, _, flowsIt, err := StreamWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("StreamWithTreeSitter failed: %v", err)
	}

	var flows []types.DataFlow
	for f := range flowsIt {
		flows = append(flows, f)
	}

	if len(flows) != 3 {
		t.Errorf("expected 3 flows, got %d", len(flows))
	}

	foundX := false
	foundYX := false
	foundY20 := false
	for _, f := range flows {
		if f.Source == "10" && f.Sink == "x" {
			foundX = true
		}
		if f.Source == "x" && f.Sink == "y" {
			foundYX = true
		}
		if f.Source == "20" && f.Sink == "y" {
			foundY20 = true
		}
	}

	if !foundX {
		t.Errorf("did not find flow x = 10")
	}
	if !foundYX {
		t.Errorf("did not find flow y = x")
	}
	if !foundY20 {
		t.Errorf("did not find flow y = 20")
	}
}

func TestStreamWithTreeSitter_ArgsAndReturns(t *testing.T) {
	t.Run("TypeScript", func(t *testing.T) {
		content := []byte(`
			function f(a, b) {
				return a + b;
			}
			const x = 10;
			const y = 20;
			f(x, y);
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pIt, _, fIt, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		for range pIt {} // Populate symbolNames

		var flows []types.DataFlow
		for f := range fIt {
			t.Logf("Found flow: Source=%s, Sink=%s, Type=%s", f.Source, f.Sink, f.Type)
			flows = append(flows, f)
		}

		foundArg0 := false
		foundArg1 := false
		foundRet := false
		for _, f := range flows {
			if f.Type == "argument" && f.Source == "x" && f.Sink == "f:arg0" {
				foundArg0 = true
			}
			if f.Type == "argument" && f.Source == "y" && f.Sink == "f:arg1" {
				foundArg1 = true
			}
			if f.Type == "return" && f.Source == "a + b" && f.Sink == "f:return0" {
				foundRet = true
			}
		}

		if !foundArg0 { t.Errorf("did not find flow x -> f:arg0") }
		if !foundArg1 { t.Errorf("did not find flow y -> f:arg1") }
		if !foundRet { t.Errorf("did not find flow a + b -> f:return0") }
	})

	t.Run("TypeScript Arrow", func(t *testing.T) {
		content := []byte(`
			const f = x => x * 2;
			f(10);
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pIt, _, fIt, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		for range pIt {} // Populate symbolNames

		var flows []types.DataFlow
		for f := range fIt {
			flows = append(flows, f)
		}

		foundArg := false
		foundRet := false
		for _, f := range flows {
			if f.Type == "argument" && f.Source == "10" && f.Sink == "f:arg0" {
				foundArg = true
			}
			// In TS arrow f = x => x * 2, f:return0 should be f.func1:return0 or similar depending on naming
			if f.Type == "return" && f.Source == "x * 2" && (f.Sink == "f:return0" || f.Sink == "f.func1:return0") {
				foundRet = true
			}
		}

		if !foundArg { t.Errorf("did not find flow 10 -> f:arg0") }
		if !foundRet { t.Errorf("did not find flow x * 2 -> f:return0") }
	})

	t.Run("Python", func(t *testing.T) {
		content := []byte(`
def f(a, b):
    return a + b

x = 10
y = 20
f(x, y)
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.py")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pIt, _, fIt, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		for range pIt {} // Populate symbolNames

		var flows []types.DataFlow
		for f := range fIt {
			t.Logf("Found flow: Source=%s, Sink=%s, Type=%s", f.Source, f.Sink, f.Type)
			flows = append(flows, f)
		}

		foundArg0 := false
		foundArg1 := false
		foundRet := false
		for _, f := range flows {
			if f.Type == "argument" && f.Source == "x" && f.Sink == "f:arg0" {
				foundArg0 = true
			}
			if f.Type == "argument" && f.Source == "y" && f.Sink == "f:arg1" {
				foundArg1 = true
			}
			if f.Type == "return" && f.Source == "a + b" && f.Sink == "f:return0" {
				foundRet = true
			}
		}

		if !foundArg0 { t.Errorf("did not find flow x -> f:arg0") }
		if !foundArg1 { t.Errorf("did not find flow y -> f:arg1") }
		if !foundRet { t.Errorf("did not find flow a + b -> f:return0") }
	})

	t.Run("Rust", func(t *testing.T) {
		content := []byte(`
fn f(a: i32, b: i32) -> i32 {
    return a + b;
}
fn main() {
    let x = 10;
    let y = 20;
    f(x, y);
}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.rs")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pIt, _, fIt, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("StreamWithTreeSitter failed: %v", err)
		}

		for range pIt {} // Populate symbolNames

		var flows []types.DataFlow
		for f := range fIt {
			t.Logf("Found flow: Source=%s, Sink=%s, Type=%s", f.Source, f.Sink, f.Type)
			flows = append(flows, f)
		}

		foundArg0 := false
		foundArg1 := false
		foundRet := false
		for _, f := range flows {
			if f.Type == "argument" && f.Source == "x" && f.Sink == "f:arg0" {
				foundArg0 = true
			}
			if f.Type == "argument" && f.Source == "y" && f.Sink == "f:arg1" {
				foundArg1 = true
			}
			if f.Type == "return" && f.Source == "a + b" && f.Sink == "f:return0" {
				foundRet = true
			}
		}

		if !foundArg0 { t.Errorf("did not find flow x -> f:arg0") }
		if !foundArg1 { t.Errorf("did not find flow y -> f:arg1") }
		if !foundRet { t.Errorf("did not find flow a + b -> f:return0") }
	})
}
