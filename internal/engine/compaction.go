package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// IdentifyCriticalContext finds high-risk symbols affected by the current diff.
func (e *CompactionEngine) IdentifyCriticalContext(ctx context.Context, diff string) ([]types.ImpactEntity, error) {
	if diff == "" {
		return nil, nil
	}

	ranges, err := parseDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var critical []types.ImpactEntity
	seen := make(map[string]bool)

	// Fetch git root for proper path resolution
	gitRoot := ""
	if rootOut, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output(); err == nil {
		gitRoot = strings.TrimSpace(string(rootOut))
	}

	impactChecks := 0

	for _, r := range ranges {
		absPath := r.Path
		if gitRoot != "" && !filepath.IsAbs(r.Path) {
			absPath = filepath.Join(gitRoot, r.Path)
		} else if !filepath.IsAbs(absPath) {
			absPath, _ = filepath.Abs(r.Path)
		}

		symbols, err := e.store.GetSymbolsByRange(ctx, absPath, r.StartLine, r.EndLine)
		if err != nil {
			continue
		}

		for _, sym := range symbols {
			key := sym.Name + ":" + sym.Path
			if seen[key] {
				continue
			}
			seen[key] = true

			// Prevent N+1 IO Thrashing: Cap the deep impact analysis to 5 symbols per diff
			if impactChecks >= 5 {
				break
			}
			impactChecks++

			impact, err := e.store.GetImpact(ctx, sym.Name, sym.Path, 3)
			if err != nil {
				continue
			}

			if impact.Target.RiskScore > 0.6 {
				critical = append(critical, impact.Target)
			}
		}
	}

	return critical, nil
}
