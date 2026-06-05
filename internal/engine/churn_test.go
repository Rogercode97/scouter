package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestAnalyzeChurn(t *testing.T) {
	// 1. Setup temporary git repo
	tmpDir, err := os.MkdirTemp("", "scouter-churn-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}

	// Workaround for go-billy v6 / Go 1.25 os.Root.Name() relative path bug in Chroot
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get CWD: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCWD)
	}()

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	runCmd("init")
	runCmd("config", "user.email", "test@example.com")
	runCmd("config", "user.name", "Test User")

	// Commit 1: file1, file2
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd("add", ".")
	runCmd("commit", "-m", "commit 1")

	// Commit 2: file2, file3
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file3.go"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd("add", ".")
	runCmd("commit", "-m", "commit 2")

	// 2. Setup store
	ctx := context.Background()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// 3. Run Churn Analysis
	engine := NewChurnEngine(s)
	if err := engine.AnalyzeChurn(ctx, tmpDir, 10); err != nil {
		t.Fatalf("AnalyzeChurn failed: %v", err)
	}

	// 4. Verify results
	// file2 changed in 2 commits (max_churn = 2)
	// file2 co-changed with file1 once and file3 once (max_co = 1)
	// file2 churn score: (2/2)*0.5 + (1/2)*0.5 = 0.5 + 0.25 = 0.75

	// Check file2
	// Let's save a dummy symbol for file2 first.
	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "file2.go", Project: "test"})
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Sym2", Path: "file2.go"})

	// Re-run analysis to update the symbol
	if err := engine.AnalyzeChurn(ctx, tmpDir, 10); err != nil {
		t.Fatalf("AnalyzeChurn failed: %v", err)
	}

	syms, err := s.GetSymbolsByNameInFile(ctx, "Sym2", "file2.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(syms) == 0 {
		t.Fatalf("Sym2 not found")
	}

	if syms[0].ChurnScore != 0.75 {
		t.Errorf("expected churn score 0.75, got %v", syms[0].ChurnScore)
	}
}
