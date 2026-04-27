package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrEmptySource indicates the resolved file contains no data.
var ErrEmptySource = errors.New("source file is empty")

// LocalFileResolver implements filter.SourceResolver for local filesystem reads.
type LocalFileResolver struct{}

// ResolveSource fetches a context window around a specific line in a file.
// It returns a 5-line window (2 before, 2 after the target line).
func (r *LocalFileResolver) ResolveSource(ctx context.Context, file string, line int) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	currentLine := 0
	
	start := line - 2
	end := line + 2
	if start < 1 {
		start = 1
	}

	for scanner.Scan() {
		currentLine++
		if currentLine >= start && currentLine <= end {
			prefix := "  "
			if currentLine == line {
				prefix = "> "
			}
			lines = append(lines, fmt.Sprintf("%d %s%s", currentLine, prefix, scanner.Text()))
		}
		if currentLine > end {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	if len(lines) == 0 {
		return "", ErrEmptySource
	}

	return strings.Join(lines, "\n"), nil
}
