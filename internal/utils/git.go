package utils

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DiffRange represents a range of modified lines in a file.
type DiffRange struct {
	Path      string
	StartLine int
	EndLine   int
}

var hunkRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// GetLocalChanges parses 'git diff HEAD' to find modified line ranges.
// Using HEAD ensures we include both staged and unstaged changes (Divine Fix).
func GetLocalChanges(ctx context.Context) ([]DiffRange, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var changes []DiffRange
	var currentFile string
	scanner := bufio.NewScanner(stdout)

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
				
				changes = append(changes, DiffRange{
					Path:      currentFile,
					StartLine: start,
					EndLine:   start + count - 1,
				})
			}
		}
	}

	_ = cmd.Wait()
	return changes, nil
}

// GetRepoName returns the name of the repository (e.g., "scouter") from origin remote.
func GetRepoName(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return ""
	}

	// Handle git@github.com:org/repo.git or https://github.com/org/repo.git
	parts := strings.Split(url, "/")
	last := parts[len(parts)-1]
	return strings.TrimSuffix(last, ".git")
}

// RestoreFile executes 'git restore <file>' to revert changes in a specific file.
func RestoreFile(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "git", "restore", path).Run()
}
