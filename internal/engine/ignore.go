package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// IgnoreMatcher reports whether a path under root is ignored by
// root .gitignore plus root .git/info/exclude. Root-only patterns on
// purpose: nested .gitignore files stay out of scope for T3.
type IgnoreMatcher struct {
	root    string
	matcher gitignore.Matcher
}

// LoadIgnoreMatcher reads root/.gitignore and root/.git/info/exclude once.
// Missing files mean "nothing ignored". It never returns nil.
func LoadIgnoreMatcher(root string) *IgnoreMatcher {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	var patterns []gitignore.Pattern
	for _, name := range []string{".gitignore", filepath.Join(".git", "info", "exclude")} {
		data, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			continue
		}
		domain := strings.Split(filepath.ToSlash(abs), "/")
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			patterns = append(patterns, gitignore.ParsePattern(trimmed, domain))
		}
	}
	return &IgnoreMatcher{root: abs, matcher: gitignore.NewMatcher(patterns)}
}

// Ignored reports true when path (file or dir) falls under an exclude.
// Paths outside root are never ignored. The root itself is never ignored.
func (m *IgnoreMatcher) Ignored(path string, isDir bool) bool {
	if m == nil || m.matcher == nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(m.root, abs)
	if err != nil || rel == "." {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	split := strings.Split(filepath.ToSlash(abs), "/")
	return m.matcher.Match(split, isDir)
}
