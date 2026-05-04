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

	ledger := NewLedger()
	engine := NewCompactionEngine(nil, ledger)
	ctx := context.Background()
	truthKernel := "Test truth kernel for compaction"

	result, err := engine.CompactSession(ctx, truthKernel)
	if err != nil {
		t.Fatalf("CompactSession failed: %v", err)
	}

	// Verify .scouter directory exists
	scouterDir := filepath.Join(tmpDir, ".scouter")
	if _, err := os.Stat(scouterDir); os.IsNotExist(err) {
		t.Errorf(".scouter directory was not created")
	}

	// Verify boundary.json exists
	boundaryPath := filepath.Join(scouterDir, "boundary.json")
	if _, err := os.Stat(boundaryPath); os.IsNotExist(err) {
		t.Errorf("boundary.json was not created")
	}

	if result.AnchorPath != boundaryPath {
		t.Errorf("expected AnchorPath %s, got %s", boundaryPath, result.AnchorPath)
	}

	// Verify content of boundary.json
	content, err := os.ReadFile(boundaryPath)
	if err != nil {
		t.Fatalf("failed to read boundary.json: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "boundary_id") {
		t.Errorf("boundary.json missing boundary_id")
	}
	if !strings.Contains(strContent, truthKernel) {
		t.Errorf("boundary.json missing truth_kernel content")
	}
}

func TestCompactSessionEmptySummary(t *testing.T) {
	ledger := NewLedger()
	engine := NewCompactionEngine(nil, ledger)
	ctx := context.Background()

	_, err := engine.CompactSession(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty summary, got nil")
	} else if err.Error() != "truth kernel cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}
