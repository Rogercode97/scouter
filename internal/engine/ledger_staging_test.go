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
