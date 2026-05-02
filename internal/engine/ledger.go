package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Patch represents a localized structural modification.
type Patch struct {
	FilePath   string
	OldContent string
	NewContent string
}

// Ledger manages a set of patches across multiple files with atomic commit/rollback.
type Ledger struct {
	mu      sync.Mutex
	patches map[string]Patch
	staged  map[string]Patch
	backups map[string][]byte
}

func NewLedger() *Ledger {
	return &Ledger{
		patches: make(map[string]Patch),
		staged:  make(map[string]Patch),
		backups: make(map[string][]byte),
	}
}

// Record adds a patch to the ledger.
func (l *Ledger) Record(filePath string, p Patch) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.patches[filePath] = p
}

// Stage adds a patch to the staging area.
func (l *Ledger) Stage(path string, p Patch) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.staged[path] = p
}

// Unstage removes a patch from the staging area.
func (l *Ledger) Unstage(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.staged, path)
}

// CommitStaged applies all staged patches to disk and clears the staging area.
func (l *Ledger) CommitStaged(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for path, patch := range l.staged {
		// Ensure backup exists for rollback if needed
		if _, exists := l.backups[path]; !exists {
			content, err := os.ReadFile(path)
			if err == nil {
				l.backups[path] = content
			}
		}

		if err := os.WriteFile(path, []byte(patch.NewContent), 0644); err != nil {
			return fmt.Errorf("failed to write staged patch to %s: %w", path, err)
		}
		// Move to committed patches
		l.patches[path] = patch
	}

	// Clear staged
	l.staged = make(map[string]Patch)
	return nil
}

// Prepare backups all affected files.
func (l *Ledger) Prepare(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for path := range l.patches {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to backup %s: %w", path, err)
		}
		l.backups[path] = content
		
		// Create .bak file on disk for extra safety
		bakPath := path + ".bak"
		if err := os.WriteFile(bakPath, content, 0644); err != nil {
			return fmt.Errorf("failed to create disk backup for %s: %w", path, err)
		}
	}
	return nil
}

// Commit applies all patches to disk.
func (l *Ledger) Commit(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for path, patch := range l.patches {
		if err := os.WriteFile(path, []byte(patch.NewContent), 0644); err != nil {
			return fmt.Errorf("failed to write patch to %s: %w", path, err)
		}
	}

	// Purge .bak files
	for path := range l.patches {
		os.Remove(path + ".bak")
	}
	return nil
}

// Rollback restores all files from backups.
func (l *Ledger) Rollback(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []string
	for path, content := range l.backups {
		if err := os.WriteFile(path, content, 0644); err != nil {
			errs = append(errs, fmt.Sprintf("failed to restore %s: %v", path, err))
		}
		os.Remove(path + ".bak")
	}

	if len(errs) > 0 {
		return fmt.Errorf("rollback errors: %s", filepath.Join(errs...))
	}
	return nil
}

func (l *Ledger) AffectedFiles() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	files := make([]string, 0, len(l.patches))
	for f := range l.patches {
		files = append(files, f)
	}
	return files
}

func (l *Ledger) StagedFiles() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	files := make([]string, 0, len(l.staged))
	for f := range l.staged {
		files = append(files, f)
	}
	return files
}
