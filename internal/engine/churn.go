package engine

import (
	"context"
	"fmt"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// ChurnStore defines the data requirements for the ChurnEngine.
type ChurnStore interface {
	store.SymbolRegistry
	store.TransactionManager
}

// ChurnEngine analyzes the git history to identify tectonic plates (co-changing files).
type ChurnEngine struct {
	store ChurnStore
}

func NewChurnEngine(s ChurnStore) *ChurnEngine {
	return &ChurnEngine{store: s}
}

// AnalyzeChurn calculates the co-change metrics and updates the store.
func (e *ChurnEngine) AnalyzeChurn(ctx context.Context, repoPath string, commitLimit int) error {
	if commitLimit <= 0 {
		commitLimit = 500
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return fmt.Errorf("failed to get log: %w", err)
	}

	fileChurn := make(map[string]int)
	coChange := make(map[string]map[string]int)
	totalCommits := 0

	err = cIter.ForEach(func(c *object.Commit) error {
		if totalCommits >= commitLimit {
			return fmt.Errorf("limit reached") // Hack to break ForEach
		}
		totalCommits++

		files, err := c.Stats()
		if err != nil {
			return nil // Skip commits with errors
		}

		var changedInCommit []string
		for _, stat := range files {
			path := stat.Name
			fileChurn[path]++
			changedInCommit = append(changedInCommit, path)
		}

		// Track co-changes (tectonic plates)
		for i := 0; i < len(changedInCommit); i++ {
			for j := i + 1; j < len(changedInCommit); j++ {
				f1, f2 := changedInCommit[i], changedInCommit[j]
				if f1 > f2 {
					f1, f2 = f2, f1
				}
				if _, ok := coChange[f1]; !ok {
					coChange[f1] = make(map[string]int)
				}
				coChange[f1][f2]++
			}
		}

		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return fmt.Errorf("failed to iterate commits: %w", err)
	}

	// Calculate scores and update symbols
	// ChurnScore = (file_churn / max_churn) * 0.5 + (max_co_change / max_churn) * 0.5
	maxChurn := 0
	for _, count := range fileChurn {
		if count > maxChurn {
			maxChurn = count
		}
	}

	if maxChurn == 0 {
		return nil
	}

	return e.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Store) error {
		for path, churn := range fileChurn {
			// Find max co-change for this file
			maxCo := 0
			// Check f1 position
			if co, ok := coChange[path]; ok {
				for _, count := range co {
					if count > maxCo {
						maxCo = count
					}
				}
			}
			// Check f2 position
			for f1, co := range coChange {
				if f1 == path {
					continue
				}
				if count, ok := co[path]; ok {
					if count > maxCo {
						maxCo = count
					}
				}
			}

			score := (float64(churn)/float64(maxChurn))*0.5 + (float64(maxCo)/float64(maxChurn))*0.5

			if err := tx.UpdateSymbolChurn(txCtx, path, score); err != nil {
				return err
			}
		}
		return nil
	})
}
