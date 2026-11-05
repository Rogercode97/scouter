package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
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
