package engine

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

var hunkRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// PredictTests identifies tests affected by changes described in the diff string.
// If diff is empty, it returns an empty list.
func PredictTests(ctx context.Context, db store.Repository, diff string) ([]types.TestTarget, error) {
	if diff == "" {
		return nil, nil
	}

	ranges, err := parseDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var allSymbols []store.Symbol
	for _, r := range ranges {
		// Normalize path to absolute to match database (Divine Fix)
		absPath, err := filepath.Abs(r.Path)
		if err != nil {
			absPath = r.Path
		}

		symbols, err := db.GetSymbolsByRange(ctx, absPath, r.StartLine, r.EndLine)
		if err != nil {
			// Skip files not in index
			continue
		}
		allSymbols = append(allSymbols, symbols...)
	}

	return findTestsForSymbols(ctx, db, allSymbols)
}

type diffRange struct {
	Path      string
	StartLine int
	EndLine   int
}

func parseDiff(diff string) ([]diffRange, error) {
	var ranges []diffRange
	var currentFile string
	scanner := bufio.NewScanner(strings.NewReader(diff))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				start, _ := strconv.Atoi(matches[1])
				count := 1
				if len(matches) == 3 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}

				ranges = append(ranges, diffRange{
					Path:      currentFile,
					StartLine: start,
					EndLine:   start + count - 1,
				})
			}
		}
	}

	return ranges, nil
}

func findTestsForSymbols(ctx context.Context, db store.Repository, symbols []store.Symbol) ([]types.TestTarget, error) {
	uniqueTests := make(map[string]types.TestTarget)
	for _, sym := range symbols {
		affectedTests, err := db.GetAffectedTests(ctx, sym.Name, sym.Path)
		if err != nil {
			return nil, err
		}
		for _, t := range affectedTests {
			key := t.Path + ":" + t.Name
			uniqueTests[key] = types.TestTarget{
				Name: t.Name,
				File: t.Path,
			}
		}
	}

	result := make([]types.TestTarget, 0, len(uniqueTests))
	for _, t := range uniqueTests {
		result = append(result, t)
	}
	return result, nil
}
