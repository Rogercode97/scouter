package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

func TestIndexRespectsGitignore(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "ignore.db")
	repo, err := store.NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer repo.Close()

	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("ignored.go\n"), 0644); err != nil {
		t.Fatalf("gitignore: %v", err)
	}
	kept := filepath.Join(tmpDir, "kept.go")
	ignored := filepath.Join(tmpDir, "ignored.go")
	if err := os.WriteFile(kept, []byte("package p\nfunc Kept() {}\n"), 0644); err != nil {
		t.Fatalf("kept: %v", err)
	}
	if err := os.WriteFile(ignored, []byte("package p\nfunc Ignored() {}\n"), 0644); err != nil {
		t.Fatalf("ignored: %v", err)
	}

	indexer := NewIndexerPipeline(IndexerConfig{Store: repo})
	if err := indexer.Index(ctx, tmpDir); err != nil {
		t.Fatalf("index: %v", err)
	}

	keptSyms, err := repo.GetSymbolsByNameInFile(ctx, "Kept", kept)
	if err != nil || len(keptSyms) == 0 {
		t.Fatalf("kept symbol missing: %v", err)
	}
	ignoredSyms, err := repo.GetSymbolsByNameInFile(ctx, "Ignored", ignored)
	if err != nil {
		t.Fatalf("query ignored: %v", err)
	}
	if len(ignoredSyms) != 0 {
		t.Fatalf("RED: ignored.go indexed with %d symbols, want 0", len(ignoredSyms))
	}
}

func TestIgnoreMatcherExcludeAndPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git", "info"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// .gitignore excludes everything, then re-includes kept.go.
	// .git/info/exclude tries to drop kept.go: the .gitignore re-include
	// must win because it loads later.
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.go\n!kept.go\n"), 0644); err != nil {
		t.Fatalf("gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".git", "info", "exclude"), []byte("kept.go\n"), 0644); err != nil {
		t.Fatalf("exclude: %v", err)
	}
	m := LoadIgnoreMatcher(tmpDir)
	if m.Ignored(filepath.Join(tmpDir, "other.go"), false) != true {
		t.Fatalf("other.go should be ignored by *.go")
	}
	if m.Ignored(filepath.Join(tmpDir, "kept.go"), false) != false {
		t.Fatalf("kept.go re-included by .gitignore must beat the exclude file")
	}
}

func TestStructuralSearchRespectsGitignore(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("ignored.go\n"), 0644); err != nil {
		t.Fatalf("gitignore: %v", err)
	}
	keptSrc := "package p\nfunc Kept() { println(1) }\n"
	ignoredSrc := "package p\nfunc Kept() { println(2) }\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "kept.go"), []byte(keptSrc), 0644); err != nil {
		t.Fatalf("kept: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ignored.go"), []byte(ignoredSrc), 0644); err != nil {
		t.Fatalf("ignored: %v", err)
	}
	rule := types.StructuralRule{Pattern: "func Kept() { println($X) }"}
	matches, err := StructuralSearchWithRule(ctx, tmpDir, rule, ".go")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, m := range matches {
		if filepath.Base(m.Path) == "ignored.go" {
			t.Fatalf("structural search hit ignored.go")
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected a match in kept.go")
	}
}
