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
	pointers, calls, err := ParseFile(ctx, filePath, nil)
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
		if call.CallerName != "Caller" {
			t.Errorf("expected caller Caller, got %s", call.CallerName)
		}
		if call.CalleeName != "Callee" {
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
	_, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	call1 := calls[0]
	if call1.CallerName != "Outer" {
		t.Errorf("expected caller Outer, got %s", call1.CallerName)
	}
	if call1.CalleeName != "Inner" {
		t.Errorf("expected callee Inner, got %s", call1.CalleeName)
	}

	// The anonymous function is now correctly tracked as Outer.func1
	call2 := calls[1]
	if call2.CallerName != "Outer.func1" {
		t.Errorf("expected caller Outer.func1 for nested call, got %s", call2.CallerName)
	}
	if call2.CalleeName != "Nested" {
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
	_, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	// Verify the first call (go func)
	call1 := calls[0]
	if call1.CallerName != "Caller.func1" {
		t.Errorf("expected caller Caller.func1, got %s", call1.CallerName)
	}
	if call1.CalleeName != "Callee" {
		t.Errorf("expected callee Callee, got %s", call1.CalleeName)
	}

	// Verify the second call (closure)
	call2 := calls[1]
	if call2.CallerName != "Caller.func2" {
		t.Errorf("expected caller Caller.func2, got %s", call2.CallerName)
	}
	if call2.CalleeName != "NestedCallee" {
		t.Errorf("expected callee NestedCallee, got %s", call2.CalleeName)
	}

	// Verify the third call (f())
	call3 := calls[2]
	if call3.CallerName != "Caller" {
		t.Errorf("expected caller Caller, got %s", call3.CallerName)
	}
	if call3.CalleeName != "f" {
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
	pointers, _, err := ParseFile(ctx, filePath, nil)
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
	_, calls, err := ParseFile(ctx, filePath, nil)
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
	pointers, _, err := ParseFile(ctx, filePath, nil)
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
