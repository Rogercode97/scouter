package engine

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-git/go-git/v5"
)

// LineProvenance tracks the origin of a specific line.
type LineProvenance struct {
	Line           int
	Commit         string
	Author         string
	EngineeringEra string
}

var (
	fixRegex      = regexp.MustCompile(`(?i)\b(fix|bug|patch)\b`)
	refactorRegex = regexp.MustCompile(`(?i)\b(refactor|perf|clean|optimize)\b`)
	featRegex     = regexp.MustCompile(`(?i)\b(feat|add|new|implement)\b`)
)

// GetFileProvenance performs a deep blame analysis using go-git
// to reconstruct the evolutionary narrative of a file.
func GetFileProvenance(ctx context.Context, repoPath, filePath string) ([]LineProvenance, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %w", err)
	}

	c, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	blame, err := git.Blame(c, filePath)
	if err != nil {
		return nil, fmt.Errorf("blame failed: %w", err)
	}

	commitCache := make(map[string]string)

	var result []LineProvenance
	for i, line := range blame.Lines {
		hashStr := line.Hash.String()
		
		era, cached := commitCache[hashStr]
		if !cached {
			cObj, err := repo.CommitObject(line.Hash)
			if err == nil {
				msg := cObj.Message
				switch {
				case fixRegex.MatchString(msg):
					era = "Fix"
				case refactorRegex.MatchString(msg):
					era = "Refactor"
				case featRegex.MatchString(msg):
					era = "Feature"
				default:
					era = "Chore"
				}
			} else {
				era = "Unknown"
			}
			commitCache[hashStr] = era
		}

		result = append(result, LineProvenance{
			Line:           i + 1,
			Commit:         hashStr[:7],
			Author:         line.Author,
			EngineeringEra: era,
		})
	}
	return result, nil
}
