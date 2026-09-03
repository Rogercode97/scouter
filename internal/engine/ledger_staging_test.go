package engine

import (
	"context"
	"os"
	"testing"
)

func TestLedgerStaging(t *testing.T) {
	l := NewLedger()
	l.SetLedgerPath("test_ledger.json")
	defer os.Remove("test_ledger.json")

	ctx := context.Background()

	path := "test_staged.txt"
	content := "staged content"
	patch := Patch{FilePath: path, NewContent: content}

	// Test Stage
	l.Stage(path, patch)
	if len(l.Staged) != 1 {
		t.Errorf("expected 1 staged patch, got %d", len(l.Staged))
	}

	// Test Unstage
	l.Unstage(path)
	if len(l.Staged) != 0 {
		t.Errorf("expected 0 staged patches after unstage, got %d", len(l.Staged))
	}

	// Test CommitStaged
	l.Stage(path, patch)
	err := l.CommitStaged(ctx)
	if err != nil {
		t.Fatalf("CommitStaged failed: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read committed file: %v", err)
	}
	if string(got) != content {
		t.Errorf("expected content %q, got %q", content, string(got))
	}

	if len(l.Staged) != 0 {
		t.Errorf("expected staged patches to be cleared after commit, got %d", len(l.Staged))
	}
}

func TestLedger_CommitPreservesMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/script.sh"
	initialContent := "#!/bin/sh\necho hi"
	fileMode := os.FileMode(0755)

	if err := os.WriteFile(path, []byte(initialContent), fileMode); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		t.Fatalf("failed to chmod file: %v", err)
	}

	l := NewLedger()
	l.SetLedgerPath(tmpDir + "/ledger.json")

	patch := Patch{
		FilePath:   path,
		NewContent: "#!/bin/sh\necho updated",
	}
	if err := l.Stage(path, patch); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	if err := l.CommitStaged(context.Background()); err != nil {
		t.Fatalf("CommitStaged failed: %v", err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file after commit: %v", err)
	}
	if stat.Mode() != fileMode {
		t.Errorf("expected mode %v, got %v", fileMode, stat.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file after commit: %v", err)
	}
	if string(data) != "#!/bin/sh\necho updated" {
		t.Errorf("expected updated content, got %q", string(data))
	}
}

func TestLedger_CommitFailure_PreservesLedgerStateAndRollsBack(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := tmpDir + "/file1.sh"
	content1 := "initial file 1"
	mode1 := os.FileMode(0755)

	if err := os.WriteFile(path1, []byte(content1), mode1); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.Chmod(path1, mode1); err != nil {
		t.Fatalf("failed to chmod file1: %v", err)
	}

	// Target that will fail apply
	badDir := tmpDir + "/bad_dir"
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatalf("failed to mkdir badDir: %v", err)
	}

	l := NewLedger()
	l.SetLedgerPath(tmpDir + "/ledger.json")

	if err := l.Stage(path1, Patch{FilePath: path1, NewContent: "mutated file 1"}); err != nil {
		t.Fatalf("failed to stage path1: %v", err)
	}
	if err := l.Stage(badDir, Patch{FilePath: badDir, NewContent: "should fail rename"}); err != nil {
		t.Fatalf("failed to stage badDir: %v", err)
	}

	err := l.CommitStaged(context.Background())
	if err == nil {
		t.Fatalf("expected CommitStaged to fail")
	}

	// 1. Ledger state MUST NOT be cleared
	if len(l.Staged) != 2 {
		t.Errorf("expected 2 staged patches in ledger after failed commit, got %d", len(l.Staged))
	}

	// 2. Modified files MUST be restored to original content and mode
	stat1, err := os.Stat(path1)
	if err != nil {
		t.Fatalf("failed to stat file1: %v", err)
	}
	if stat1.Mode() != mode1 {
		t.Errorf("expected mode %v for file1, got %v", mode1, stat1.Mode())
	}
	read1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("failed to read file1: %v", err)
	}
	if string(read1) != content1 {
		t.Errorf("expected file1 restored to %q, got %q", content1, string(read1))
	}

	// 3. Staging tmp files must not leak
	if _, err := os.Stat(path1 + ".scouter.tmp"); !os.IsNotExist(err) {
		t.Errorf("leaked tmp file for file1")
	}
	if _, err := os.Stat(badDir + ".scouter.tmp"); !os.IsNotExist(err) {
		t.Errorf("leaked tmp file for badDir")
	}
}
