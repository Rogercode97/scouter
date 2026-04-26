package utils

import (
	"fmt"
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

	// MANDATE: Reject absolute paths unless they belong to a verified safe temp dir.
	// We pin the allowed temp dir to the system default to prevent TMPDIR injection attacks.
	systemTmp := os.TempDir()
	
	if filepath.IsAbs(path) {
		if !strings.HasPrefix(path, systemTmp) {
			return "", fmt.Errorf("security violation: absolute paths outside /tmp are prohibited (%s)", path)
		}
	}

	// Construct candidate absolute path
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(root, path)
	}

	// 🔱 SECURE SYMLINK RESOLUTION (Recursive Fallback)
	// We resolve symlinks of the existing part of the path to prevent TOCTOU/Escape tricks.
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// File doesn't exist, we must validate its existing parent recursively.
		parent := filepath.Dir(fullPath)
		for {
			if rp, err := filepath.EvalSymlinks(parent); err == nil {
				// Parent exists and is resolved. Check if it's within bounds.
				if !isWithinSovereignty(rp, root, systemTmp) {
					return "", fmt.Errorf("security violation: path parent escapes sovereignty (%s)", rp)
				}
				break
			}
			nextParent := filepath.Dir(parent)
			if nextParent == parent {
				break // Root reached
			}
			parent = nextParent
		}
		realPath = filepath.Clean(fullPath)
	}

	// 🏛️ SOVEREIGNTY BOUNDARY CHECK
	if !isWithinSovereignty(realPath, root, systemTmp) {
		return "", fmt.Errorf("security violation: access denied to path outside sovereignty (%s)", realPath)
	}

	// 💎 RELATIVE BLACKLIST CHECK (Case-Insensitive)
	// We only check the parts relative to the root to avoid "Parent Pollution".
	var relPath string
	if strings.HasPrefix(realPath, root) {
		relPath, _ = filepath.Rel(root, realPath)
	} else {
		relPath, _ = filepath.Rel(systemTmp, realPath)
	}

	parts := strings.Split(relPath, string(os.PathSeparator))
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
