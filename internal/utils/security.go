package utils

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// purityBlacklist contains names that are strictly prohibited within the sovereign roots.
var purityBlacklist = []string{
	".git", ".ssh", ".env", ".scouter",
	"node_modules", "vendor", "dist", "build",
	".vscode", ".idea", ".DS_Store",
}

// GetRepoRoot locates the dynamic anchor of the project (go.mod or .git).
func GetRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	curr := cwd
	for {
		if _, err := os.Stat(filepath.Join(curr, "go.mod")); err == nil {
			return curr, nil
		}
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return cwd, nil
}

// ValidatePath implements "Project Jail" (CWE-22 prevention) with absolute signal.
func ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	root, err := GetRepoRoot()
	if err != nil {
		return "", err
	}

	systemTmp := os.TempDir()
	
	// 1. Normalize to absolute path
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(root, path)
	}

	// 2. 🔱 SECURE SYMLINK RESOLUTION
	// We must resolve symlinks even for non-existent paths to prevent jailbreaks.
	// We find the deepest existing parent and resolve its symlinks.
	realPath := absPath
	current := absPath
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err == nil {
				suffix, _ := filepath.Rel(current, absPath)
				realPath = filepath.Join(resolved, suffix)
			}
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	realPath = filepath.Clean(realPath)

	// 3. 🏛️ SOVEREIGNTY BOUNDARY CHECK
	if !isWithinSovereignty(realPath, root, systemTmp) {
		// Specific error for absolute paths to satisfy existing tests
		if filepath.IsAbs(path) {
			return "", fmt.Errorf("security violation: absolute paths outside project root or /tmp are prohibited")
		}
		return "", fmt.Errorf("security violation: path escapes sovereignty (%s)", realPath)
	}

	// 4. 💎 BLACKLIST CHECK (Case-Insensitive)
	parts := strings.Split(realPath, string(os.PathSeparator))
	for _, part := range parts {
		for _, blocked := range purityBlacklist {
			if strings.EqualFold(part, blocked) {
				return "", fmt.Errorf("purity violation: access to %s is prohibited", blocked)
			}
		}
	}

	return realPath, nil
}

// isWithinSovereignty checks if a resolved path is strictly inside the repo root or system temp.
func isWithinSovereignty(path, root, tmp string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	tmp = filepath.Clean(tmp)

	inRepo := strings.HasPrefix(path, root+string(os.PathSeparator)) || path == root
	inTmp := strings.HasPrefix(path, tmp+string(os.PathSeparator)) || path == tmp
	
	return inRepo || inTmp
}

// SanitizeFTS sanitizes a raw search string for safe use in SQLite FTS5 MATCH expressions.
// It escapes double quotes and wraps the query in double quotes to neutralize control characters.
func SanitizeFTS(q string) string {
	if q == "" {
		return ""
	}
	
	// Check for trailing wildcard
	hasWildcard := strings.HasSuffix(q, "*")
	
	// 1. Clean the string
	s := strings.TrimSpace(q)
	s = strings.TrimSuffix(s, "*")
	
	// 2. Escape double quotes (FTS5 uses double double-quotes for literal quotes)
	s = strings.ReplaceAll(s, "\"", "\"\"")
	
	// 3. Remove leading wildcards (SQLite doesn't support them at the start of a term)
	s = strings.TrimLeft(s, "*")
	
	if s == "" {
		return ""
	}

	// 4. Wrap in quotes to neutralize OR, AND, NEAR, etc.
	res := "\"" + s + "\""
	if hasWildcard {
		res += "*"
	}
	
	return res
}

// SafeUintToInt safely converts a uint to an int, preventing overflow on 32-bit systems.
func SafeUintToInt(u uint) (int, error) {
	if uint64(u) > uint64(math.MaxInt) {
		return 0, fmt.Errorf("integer overflow: %d exceeds maximum integer value", u)
	}
	return int(u), nil
}
