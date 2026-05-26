package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestChronosEngine(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")

	// 1. Initial State
	initialContent := `package test

func MyFunction() {
	// do something
}

type MyStruct struct {}

func (m *MyStruct) MyMethod() {}
`
	if err := os.WriteFile(filePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	chronos := NewChronosEngine()
	ctx := context.Background()

	// 2. Take Snapshot
	snapshot, err := chronos.TakeSnapshot(ctx, filePath)
	if err != nil {
		t.Fatalf("Failed to take snapshot: %v", err)
	}

	if len(snapshot.Symbols) != 2 {
		t.Errorf("Expected 2 symbols (MyFunction, MyMethod), got %d", len(snapshot.Symbols))
	}

	// 3. Unchanged Comparison (White space changes only)
	unchangedContent := `package test

func MyFunction() {
	// do something

}

type MyStruct struct {}

func (m *MyStruct) MyMethod() {

}
`
	if err := os.WriteFile(filePath, []byte(unchangedContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	diff, err := chronos.CompareSnapshot(ctx, snapshot, filePath)
	if err != nil {
		t.Fatalf("Failed to compare unchanged snapshot: %v", err)
	}

	if diff.Unchanged != 2 {
		t.Errorf("Expected 2 unchanged symbols, got %d", diff.Unchanged)
	}
	if len(diff.MangledSymbols) > 0 || len(diff.MissingSymbols) > 0 || len(diff.AddedSymbols) > 0 {
		t.Errorf("Expected clean diff, got Missing:%d Mangled:%d Added:%d", len(diff.MissingSymbols), len(diff.MangledSymbols), len(diff.AddedSymbols))
	}

	// 4. Internal Logic Change (body mutation — the key Chronos test)
	logicChangedContent := `package test

func MyFunction() {
	panic("injected bug")
}

type MyStruct struct {}

func (m *MyStruct) MyMethod() {}
`
	if err := os.WriteFile(filePath, []byte(logicChangedContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	diffLogic, err := chronos.CompareSnapshot(ctx, snapshot, filePath)
	if err != nil {
		t.Fatalf("Failed to compare logic-changed snapshot: %v", err)
	}

	if len(diffLogic.MangledSymbols) != 1 || diffLogic.MangledSymbols[0] != "MyFunction" {
		t.Errorf("Expected MyFunction as mangled (body changed), got Mangled:%v Missing:%v", diffLogic.MangledSymbols, diffLogic.MissingSymbols)
	}
	if diffLogic.Unchanged != 1 {
		t.Errorf("Expected 1 unchanged (MyMethod), got %d", diffLogic.Unchanged)
	}

	// 5. Structural Breakage (Delete method, add new one)
	brokenContent := `package test

func MyFunction() {
	panic("changed structural logic")
}

type MyStruct struct {}

func NewFunction() {}
`
	if err := os.WriteFile(filePath, []byte(brokenContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	diff2, err := chronos.CompareSnapshot(ctx, snapshot, filePath)
	if err != nil {
		t.Fatalf("Failed to compare broken snapshot: %v", err)
	}

	if len(diff2.MissingSymbols) != 1 || diff2.MissingSymbols[0] != "MyMethod" {
		t.Errorf("Expected Missing MyMethod, got %v", diff2.MissingSymbols)
	}

	if len(diff2.AddedSymbols) != 1 || diff2.AddedSymbols[0] != "NewFunction" {
		t.Errorf("Expected Added NewFunction, got %v", diff2.AddedSymbols)
	}

	// MyFunction should be mangled (body changed from original)
	if len(diff2.MangledSymbols) != 1 || diff2.MangledSymbols[0] != "MyFunction" {
		t.Errorf("Expected MyFunction as mangled, got %v", diff2.MangledSymbols)
	}
}
