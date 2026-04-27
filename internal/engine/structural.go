package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// StructuralMatch represents a match found by structural search.
type StructuralMatch struct {
	Path      string      `json:"path"`
	Range     types.Range `json:"range"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Content   string      `json:"content"`
}

// StructuralSearch searches for a pattern in a file or directory.
func StructuralSearch(ctx context.Context, rootPath, pattern, ext string) ([]StructuralMatch, error) {
	lang, ok := languageConfigs[ext]
	if !ok {
		// Try to find by extension if ext is just an extension
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		lang, ok = languageConfigs[ext]
		if !ok {
			return nil, nil // Unsupported language
		}
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang.Language)

	// Pre-process pattern to make wildcards valid identifiers
	// E.g., $X -> __SCT_X__
	processedPattern := pattern
	processedPattern = strings.ReplaceAll(processedPattern, "$$$", "__SCT_MULTI__")
	for i := 'A'; i <= 'Z'; i++ {
		processedPattern = strings.ReplaceAll(processedPattern, "$"+string(i), "__SCT_"+string(i)+"__")
	}

	// Pattern Parsing Strategy: Try as-is, then try wrapped if ERROR found
	var patternTree *tree_sitter.Tree
	patternTree = parser.Parse([]byte(processedPattern), nil)
	patternRoot := patternTree.RootNode()
	pContent := []byte(processedPattern)

	if patternRoot.HasError() {
		// Try wrapping in a function body for Go/TS
		wrapped := processedPattern
		if ext == ".go" {
			wrapped = "package main\nfunc _() {\n" + processedPattern + "\n}"
		} else if ext == ".ts" || ext == ".js" {
			wrapped = "function _() {\n" + processedPattern + "\n}"
		}

		wTree := parser.Parse([]byte(wrapped), nil)
		if !wTree.RootNode().HasError() {
			pContent = []byte(wrapped)
			patternRoot = findInnermostMatch(wTree.RootNode(), pContent, processedPattern)
			patternTree.Close() // Close original tree with error
			patternTree = wTree // Keep wTree for the search
		} else {
			wTree.Close()
		}
	}
	defer patternTree.Close()

	// Extract effective node (skip source_file wrapper if it's just one child)
	for patternRoot != nil && patternRoot.ChildCount() == 1 && !isWildcard(patternRoot.Utf8Text(pContent)) {
		patternRoot = patternRoot.Child(0)
	}

	if patternRoot == nil {
		return nil, nil
	}

	var matches []StructuralMatch
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr // Context Sovereignty
		}

		if err != nil || info.IsDir() || filepath.Ext(path) != ext {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Use anonymous function to safely defer tree.Close() (Strike 1 Redemption)
		func() {
			tree := parser.Parse(content, nil)
			defer tree.Close()
			findMatches(tree.RootNode(), patternRoot, pContent, content, path, &matches)
		}()
		return nil
	})

	return matches, err
}

func findInnermostMatch(node *tree_sitter.Node, content []byte, pattern string) *tree_sitter.Node {
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		child := node.Child(i)
		// Skip comments and whitespace-only nodes
		if child.Kind() == "comment" || strings.TrimSpace(child.Utf8Text(content)) == "" {
			continue
		}
		if strings.Contains(child.Utf8Text(content), strings.TrimSpace(pattern)) {
			return findInnermostMatch(child, content, pattern)
		}
	}
	return node
}

func findMatches(node, pattern *tree_sitter.Node, patternContent, targetContent []byte, path string, matches *[]StructuralMatch) {
	if matchNodes(node, pattern, patternContent, targetContent) {
		*matches = append(*matches, StructuralMatch{
			Path:      path,
			Range:     types.Range{Start: int(node.StartByte()), End: int(node.EndByte())},
			StartLine: int(node.StartPosition().Row) + 1,
			EndLine:   int(node.EndPosition().Row) + 1,
			Content:   node.Utf8Text(targetContent),
		})
	}

	for i := uint(0); i < uint(node.ChildCount()); i++ {
		findMatches(node.Child(i), pattern, patternContent, targetContent, path, matches)
	}
}

func isWildcard(text string) bool {
	return strings.HasPrefix(text, "__SCT_")
}

func matchNodes(node, pattern *tree_sitter.Node, patternContent, targetContent []byte) bool {
	pText := pattern.Utf8Text(patternContent)
	
	if isWildcard(pText) {
		return true
	}

	if node.Kind() != pattern.Kind() {
		return false
	}

	if pattern.ChildCount() == 0 {
		return node.Utf8Text(targetContent) == pText
	}

	if node.ChildCount() < pattern.ChildCount() {
		return false
	}

	pIdx := uint(0)
	for tIdx := uint(0); tIdx < uint(node.ChildCount()) && pIdx < uint(pattern.ChildCount()); tIdx++ {
		if matchNodes(node.Child(tIdx), pattern.Child(pIdx), patternContent, targetContent) {
			pIdx++
		}
	}

	return pIdx == uint(pattern.ChildCount())
}
