package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileWithCalls(t *testing.T) {
	content := `
package test
func Caller() {
	Callee()
}
func Callee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	pointers, calls, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify pointers (symbols)
	if len(pointers) != 2 {
		t.Errorf("expected 2 pointers, got %d", len(pointers))
	}

	// Verify calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(calls))
	} else {
		call := calls[0]
		if call.CallerName != "command-line-arguments.Caller" {
			t.Errorf("expected caller Caller, got %s", call.CallerName)
		}
		if call.CalleeName != "command-line-arguments.Callee" {
			t.Errorf("expected callee Callee, got %s", call.CalleeName)
		}
	}
}

func TestParseFileWithNestedCalls(t *testing.T) {
	content := `
package test
func Outer() {
	Inner()
	func() {
		Nested()
	}()
}
func Inner() {}
func Nested() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 2 {
		for i, c := range calls {
			t.Logf("Call %d: %s -> %s", i, c.CallerName, c.CalleeName)
		}
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	call1 := calls[0]
	if call1.CallerName != "command-line-arguments.Outer" {
		t.Errorf("expected caller Outer, got %s", call1.CallerName)
	}
	if call1.CalleeName != "command-line-arguments.Inner" {
		t.Errorf("expected callee Inner, got %s", call1.CalleeName)
	}

	// The anonymous function is now correctly tracked as Outer.func1
	call2 := calls[1]
	if call2.CallerName != "command-line-arguments.Outer.func1" {
		t.Errorf("expected caller Outer.func1 for nested call, got %s", call2.CallerName)
	}
	if call2.CalleeName != "command-line-arguments.Nested" {
		t.Errorf("expected callee Nested, got %s", call2.CalleeName)
	}
}

func TestParseFileWithAnonymousCalls(t *testing.T) {
	content := `
package test
func Caller() {
	go func() {
		Callee()
	}()

	f := func() {
		NestedCallee()
	}
	f()
}
func Callee() {}
func NestedCallee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	// Verify the first call (go func)
	call1 := calls[0]
	if call1.CallerName != "command-line-arguments.Caller.func1" {
		t.Errorf("expected caller Caller.func1, got %s", call1.CallerName)
	}
	if call1.CalleeName != "command-line-arguments.Callee" {
		t.Errorf("expected callee Callee, got %s", call1.CalleeName)
	}

	// Verify the second call (closure)
	call2 := calls[1]
	if call2.CallerName != "command-line-arguments.Caller.func2" {
		t.Errorf("expected caller Caller.func2, got %s", call2.CallerName)
	}
	if call2.CalleeName != "command-line-arguments.NestedCallee" {
		t.Errorf("expected callee NestedCallee, got %s", call2.CalleeName)
	}

	// Verify the third call (f())
	call3 := calls[2]
	if call3.CallerName != "command-line-arguments.Caller" {
		t.Errorf("expected caller Caller, got %s", call3.CallerName)
	}
	if call3.CalleeName != "command-line-arguments.f" {
		t.Errorf("expected callee f, got %s", call3.CalleeName)
	}
}

func TestParseFileWithDoc(t *testing.T) {
	content := `
package test

// Hello is a greeting function.
func Hello() {}

/*
World is a global function.
It does something.
*/
func World() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	pointers, _, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(pointers) != 2 {
		t.Fatalf("expected 2 pointers, got %d", len(pointers))
	}

	if pointers[0].Name == "Hello" {
		expected := "Hello is a greeting function."
		if pointers[0].Doc != expected {
			t.Errorf("expected doc %q, got %q", expected, pointers[0].Doc)
		}
	} else {
		t.Errorf("expected first pointer to be Hello")
	}

	if pointers[1].Name == "World" {
		expected := "World is a global function.\nIt does something."
		if pointers[1].Doc != expected {
			t.Errorf("expected doc %q, got %q", expected, pointers[1].Doc)
		}
	} else {
		t.Errorf("expected second pointer to be World")
	}
}

func TestParseTypeScriptWithCalls(t *testing.T) {
	content := `
function caller() {
	callee();
}
function callee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.ts")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(calls))
	} else {
		call := calls[0]
		if call.CallerName != "caller" {
			t.Errorf("expected caller caller, got %s", call.CallerName)
		}
		if call.CalleeName != "callee" {
			t.Errorf("expected callee callee, got %s", call.CalleeName)
		}
	}
}

func TestParseFileSemantic(t *testing.T) {
	content := `
package testpkg

type MyStruct struct{}

func (s MyStruct) ValueMethod() {}
func (s *MyStruct) PointerMethod() {}
func GlobalFunc() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-semantic-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy go.mod so packages.Load works
	goMod := "module testpkg\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	pointers, _, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	foundValue := false
	foundPointer := false
	foundGlobal := false
	foundStruct := false

	for _, p := range pointers {
		switch p.Name {
		case "MyStruct.ValueMethod":
			foundValue = true
			if p.ReceiverType != "value" {
				t.Errorf("ValueMethod: expected receiver_type value, got %s", p.ReceiverType)
			}
			if p.PackagePath == "" {
				t.Errorf("ValueMethod: expected package_path, got empty")
			}
		case "MyStruct.PointerMethod":
			foundPointer = true
			if p.ReceiverType != "pointer" {
				t.Errorf("PointerMethod: expected receiver_type pointer, got %s", p.ReceiverType)
			}
		case "GlobalFunc":
			foundGlobal = true
			if p.ReceiverType != "" {
				t.Errorf("GlobalFunc: expected empty receiver_type, got %s", p.ReceiverType)
			}
		case "MyStruct":
			foundStruct = true
			if p.Type != "class" {
				t.Errorf("MyStruct: expected type class, got %s", p.Type)
			}
		}
	}

	if !foundValue {
		t.Errorf("ValueMethod not found")
	}
	if !foundPointer {
		t.Errorf("PointerMethod not found")
	}
	if !foundGlobal {
		t.Errorf("GlobalFunc not found")
	}
	if !foundStruct {
		t.Errorf("MyStruct not found")
	}
}

func TestParseFileWithDataFlow(t *testing.T) {
	content := `
package test
func Flow() {
	x := 42
	y := x
	z := y
	_ = z
}
`
	tmpDir, _ := os.MkdirTemp("", "scouter-flow-*")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	os.WriteFile(filePath, []byte(content), 0644)

	ctx := context.Background()
	_, _, flows, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(flows) != 3 {
		t.Errorf("expected 3 flows, got %d", len(flows))
	}

	foundXY := false
	foundYZ := false
	for _, f := range flows {
		if f.Source == "42" && f.Sink == "x" {
			foundXY = true
		}
		if f.Source == "x" && f.Sink == "y" {
			foundYZ = true
		}
	}

	if !foundXY {
		t.Errorf("did not find flow x := 42")
	}
	if !foundYZ {
		t.Errorf("did not find flow y := x")
	}
}

func TestParseFileWithArgsAndReturns(t *testing.T) {
	content := `
package test
func F(a, b int) int {
	return a + b
}
func G() (int, int) {
	return 1, 2
}
func Main() {
	x := 10
	y := 20
	z := F(x, y)
	a, b := G()
	F(G())
	_ = z
	_ = a
	_ = b
}
`
	tmpDir, _ := os.MkdirTemp("", "scouter-args-*")
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	os.WriteFile(filePath, []byte(content), 0644)

	ctx := context.Background()
	_, _, flows, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify Arguments
	foundArg0 := false
	foundArg1 := false
	for _, f := range flows {
		if f.Type == "argument" && f.Source == "x" && f.Sink == "F:arg0" {
			foundArg0 = true
		}
		if f.Type == "argument" && f.Source == "y" && f.Sink == "F:arg1" {
			foundArg1 = true
		}
	}
	if !foundArg0 {
		t.Errorf("did not find flow x -> F:arg0")
	}
	if !foundArg1 {
		t.Errorf("did not find flow y -> F:arg1")
	}

	// Verify Returns
	foundRetF := false
	foundRetG0 := false
	foundRetG1 := false
	for _, f := range flows {
		if f.Type == "return" && f.Source == "a + b" && f.Sink == "F:return0" {
			foundRetF = true
		}
		if f.Type == "return" && f.Source == "1" && f.Sink == "G:return0" {
			foundRetG0 = true
		}
		if f.Type == "return" && f.Source == "2" && f.Sink == "G:return1" {
			foundRetG1 = true
		}
	}
	if !foundRetF {
		t.Errorf("did not find flow a+b -> F:return0")
	}
	if !foundRetG0 {
		t.Errorf("did not find flow 1 -> G:return0")
	}
	if !foundRetG1 {
		t.Errorf("did not find flow 2 -> G:return1")
	}

	// Verify Assignment Return Mapping
	foundZ := false
	foundA := false
	foundB := false
	for _, f := range flows {
		if f.Type == "assignment" && f.Source == "F:return0" && f.Sink == "z" {
			foundZ = true
		}
		if f.Type == "assignment" && f.Source == "G:return0" && f.Sink == "a" {
			foundA = true
		}
		if f.Type == "assignment" && f.Source == "G:return1" && f.Sink == "b" {
			foundB = true
		}
	}
	if !foundZ {
		t.Errorf("did not find flow F:return0 -> z")
	}
	if !foundA {
		t.Errorf("did not find flow G:return0 -> a")
	}
	if !foundB {
		t.Errorf("did not find flow G:return1 -> b")
	}

	// Verify Nested Call Mapping: F(G())
	// Note: Our current implementation in parser.go handles G() as a single argument to F.
	// Task 4 says "link function call returns to assignment targets". 
	// Task 2 says "emit argument flows for each parameter (e.g., x -> f:arg0)".
	// In F(G()), G() is the argument.
	foundNestedArg := false
	for _, f := range flows {
		if f.Type == "argument" && f.Source == "G()" && f.Sink == "F:arg0" {
			foundNestedArg = true
		}
	}
	if !foundNestedArg {
		t.Errorf("did not find flow G() -> F:arg0")
	}
}
