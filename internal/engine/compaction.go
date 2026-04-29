package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// CompactionEngine handles context hygiene and latent memory anchoring.
type CompactionEngine struct {
	store store.Repository
}

func NewCompactionEngine(s store.Repository) *CompactionEngine {
	return &CompactionEngine{store: s}
}

// CompactSession generates a technical summary and saves it to a persistent anchor.
func (e *CompactionEngine) CompactSession(ctx context.Context, summary string) (*types.CompactionResult, error) {
	if summary == "" {
		return nil, fmt.Errorf("summary cannot be empty")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	scouterDir := filepath.Join(cwd, ".scouter")
	if err := os.MkdirAll(scouterDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .scouter directory: %w", err)
	}

	anchorPath := filepath.Join(scouterDir, "anchor.md")
	
	timestamp := time.Now().Format(time.RFC3339)
	content := fmt.Sprintf("# Scouter Session Anchor\n\n**Timestamp**: %s\n\n## Technical State\n%s\n", timestamp, summary)

	if err := os.WriteFile(anchorPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write anchor file: %w", err)
	}

	return &types.CompactionResult{
		AnchorPath: anchorPath,
		Timestamp:  timestamp,
		Message:    "Context compacted successfully. You can now start a fresh session and read .scouter/anchor.md to resume.",
	}, nil
}
