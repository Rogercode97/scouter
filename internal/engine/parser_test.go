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
	pointers, calls, err := ParseFile(ctx, filePath)
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
	_, calls, err := ParseFile(ctx, filePath)
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

	// Note: The current Go parser implementation does not handle anonymous functions as callers.
	// This is a known limitation. We are checking for the call to Nested() inside the Outer() scope.
	call2 := calls[1]
	if call2.CallerName != "Outer" {
		t.Errorf("expected caller Outer for nested call, got %s", call2.CallerName)
	}
	if call2.CalleeName != "Nested" {
		t.Errorf("expected callee Nested, got %s", call2.CalleeName)
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
	_, calls, err := ParseFile(ctx, filePath)
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
