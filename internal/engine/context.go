package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
)

type ContextStore interface {
	GraphStore
}

// ContextEngine synthesizes single-shot architectural context packets for AI agents.
type ContextEngine struct {
	store ContextStore
}

func NewContextEngine(s ContextStore) *ContextEngine {
	return &ContextEngine{store: s}
}

// BuildFileContext returns an ultra-compact single-call context packet for a given file.
func (e *ContextEngine) BuildFileContext(ctx context.Context, filePath string) (*types.FileContextResult, error) {
	cleanPath := filepath.Clean(filePath)

	// 1. Calculate file LOC
	loc := 0
	if f, err := os.Open(cleanPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			loc++
		}
		_ = f.Close()
	}

	// 2. Fetch symbols belonging to this file
	symbols, err := e.store.GetSymbolsByPathPrefix(ctx, cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch symbols for file %s: %w", cleanPath, err)
	}

	// Filter strictly for this file in case path prefix matched sibling files
	var fileSymbols []string
	maxChurn := 0.0
	for _, sym := range symbols {
		if filepath.Clean(sym.Path) == cleanPath {
			fileSymbols = append(fileSymbols, sym.Name)
			if sym.EndLine > loc {
				loc = sym.EndLine
			}
			if sym.ChurnScore > maxChurn {
				maxChurn = sym.ChurnScore
			}
		}
	}

	// 3. Collect direct and transitive dependents + affected tests
	directDepSet := make(map[string]bool)
	transitiveSet := make(map[string]bool)
	testHintSet := make(map[string]bool)

	for _, symName := range fileSymbols {
		// Direct callers
		if callers, err := e.store.GetCallers(ctx, symName, 100, 0); err == nil {
			for _, c := range callers {
				callerPath := filepath.Clean(c.Path)
				if callerPath != "" && callerPath != cleanPath {
					directDepSet[callerPath] = true
					transitiveSet[callerPath] = true
				}
			}
		}

		// Transitive callers
		if recCallers, err := e.store.GetCallersRecursive(ctx, symName, cleanPath, 3); err == nil {
			for _, rc := range recCallers {
				callerPath := filepath.Clean(rc.Path)
				if callerPath != "" && callerPath != cleanPath {
					transitiveSet[callerPath] = true
				}
			}
		}

		// Affected tests
		if tests, err := e.store.GetAffectedTestsRecursive(ctx, symName, cleanPath); err == nil {
			for _, t := range tests {
				if t.Path != "" {
					testHintSet[filepath.Clean(t.Path)] = true
				}
			}
		}
	}

	// 4. Companion test file check on disk
	if strings.HasSuffix(cleanPath, ".go") && !strings.HasSuffix(cleanPath, "_test.go") {
		candidate := strings.TrimSuffix(cleanPath, ".go") + "_test.go"
		if _, err := os.Stat(candidate); err == nil {
			testHintSet[filepath.Clean(candidate)] = true
		}
	}

	// 5. Flatten and sort maps to deterministic slices
	directDeps := make([]string, 0, len(directDepSet))
	for p := range directDepSet {
		directDeps = append(directDeps, p)
	}
	sort.Strings(directDeps)

	transitiveDeps := make([]string, 0, len(transitiveSet))
	for p := range transitiveSet {
		transitiveDeps = append(transitiveDeps, p)
	}
	sort.Strings(transitiveDeps)

	testHints := make([]string, 0, len(testHintSet))
	for p := range testHintSet {
		testHints = append(testHints, p)
	}
	sort.Strings(testHints)

	// 6. Compute EditCost
	filesCount := len(directDeps)
	cascadeCount := len(transitiveDeps)
	estimatedTokens := (loc * 4) + (filesCount * 250)

	risk := "low"
	if filesCount >= 5 || maxChurn >= 0.7 || cascadeCount >= 10 {
		risk = "high"
	} else if filesCount >= 2 || maxChurn >= 0.3 || cascadeCount >= 4 {
		risk = "medium"
	}

	verdict := "safe"
	if risk == "high" {
		verdict = "danger"
	} else if risk == "medium" {
		verdict = "caution"
	}

	return &types.FileContextResult{
		File:             cleanPath,
		LOC:              loc,
		Symbols:          len(fileSymbols),
		DirectDependents: directDeps,
		TransitiveImpact: cascadeCount,
		ChurnScore:       maxChurn,
		EditCost: types.EditCost{
			Files:   filesCount,
			Cascade: cascadeCount,
			Tokens:  estimatedTokens,
			Risk:    risk,
		},
		TestHints: testHints,
		Verdict:   verdict,
	}, nil
}
