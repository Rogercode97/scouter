package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockStore implements store.Repository for testing.
type MockStore struct{}

func (m *MockStore) GetFileIndex(ctx context.Context, path string) (*any, error) { return nil, nil }
// Implementing the minimum required to satisfy the interface if needed, 
// but since CompactSession doesn't use it yet, we just need it to exist.
// Actually, I'll just use a nil or a minimal struct if it's just for the constructor.

func TestCompactSession(t *testing.T) {
	// Setup temporary working directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current wd: %v", err)
	}
	
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change wd: %v", err)
	}
	defer os.Chdir(oldCwd)

	engine := NewCompactionEngine(nil)
	ctx := context.Background()
	summary := "Test summary for compaction"

	result, err := engine.CompactSession(ctx, summary)
	if err != nil {
		t.Fatalf("CompactSession failed: %v", err)
	}

	// Verify .scouter directory exists
	scouterDir := filepath.Join(tmpDir, ".scouter")
	if _, err := os.Stat(scouterDir); os.IsNotExist(err) {
		t.Errorf(".scouter directory was not created")
	}

	// Verify anchor.md exists
	anchorPath := filepath.Join(scouterDir, "anchor.md")
	if _, err := os.Stat(anchorPath); os.IsNotExist(err) {
		t.Errorf("anchor.md was not created")
	}

	if result.AnchorPath != anchorPath {
		t.Errorf("expected AnchorPath %s, got %s", anchorPath, result.AnchorPath)
	}

	// Verify content of anchor.md
	content, err := os.ReadFile(anchorPath)
	if err != nil {
		t.Fatalf("failed to read anchor.md: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "# Scouter Session Anchor") {
		t.Errorf("anchor.md missing header")
	}
	if !strings.Contains(strContent, "**Timestamp**:") {
		t.Errorf("anchor.md missing timestamp")
	}
	if !strings.Contains(strContent, summary) {
		t.Errorf("anchor.md missing summary content")
	}
}

func TestCompactSessionEmptySummary(t *testing.T) {
	engine := NewCompactionEngine(nil)
	ctx := context.Background()

	_, err := engine.CompactSession(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty summary, got nil")
	} else if err.Error() != "summary cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}
