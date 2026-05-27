package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestZeroLatencyBypass(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "scouter-perf-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "perf.db")
	repo, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer repo.Close()

	engine := NewTruthEngine(repo)

	// 1. Create a "large" structure
	subDir := filepath.Join(tmpDir, "pkg", "core")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}

	files := []string{
		filepath.Join(tmpDir, "main.go"),
		filepath.Join(subDir, "logic.go"),
		filepath.Join(subDir, "utils.go"),
	}

	content := "package main\nfunc Test() {}"
	for _, f := range files {
		if err := os.WriteFile(f, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", f, err)
		}
	}

	// 2. First Index (Cold)
	start := time.Now()
	err = engine.Index(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Cold index failed: %v", err)
	}
	coldDuration := time.Since(start)
	t.Logf("Cold Index Duration: %v", coldDuration)

	// 3. Second Index (Hot - No changes)
	// We wait a bit to ensure mtime precision isn't an issue in fast systems
	time.Sleep(10 * time.Millisecond)
	
	start = time.Now()
	err = engine.Index(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Hot index failed: %v", err)
	}
	hotDuration := time.Since(start)
	t.Logf("Hot Index Duration (Bypass): %v", hotDuration)

	if hotDuration > coldDuration/2 && hotDuration > 5*time.Millisecond {
		t.Errorf("Hot index (%v) is not significantly faster than cold index (%v). Bypass might not be working.", hotDuration, coldDuration)
	}

	// 4. Modify one file
	time.Sleep(10 * time.Millisecond)
	modifiedFile := files[1]
	if err := os.WriteFile(modifiedFile, []byte("package main\nfunc NewLogic() {}"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	start = time.Now()
	err = engine.Index(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Modified index failed: %v", err)
	}
	modDuration := time.Since(start)
	t.Logf("Modified Index Duration: %v", modDuration)

	// Verify that the new symbol exists
	syms, err := repo.GetSymbolsByNameInFile(ctx, "NewLogic", modifiedFile)
	if err != nil || len(syms) == 0 {
		t.Errorf("New symbol not found after re-indexing modified branch")
	}
}