package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Rogercode97/scouter/internal/types"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var (
	structOnce sync.Once
)

// ensureStructuralInit ensures that any structural-search specific initialization is done.
// In this case, we rely on the languages registered in treesitter.go.
func ensureStructuralInit() {
	structOnce.Do(func() {
		// Languages are already registered in init() of treesitter.go
		// but we can add additional setup here if needed.
	})
}

// StructuralMatch represents a match found by structural search.
type StructuralMatch struct {
	Path      string            `json:"path"`
	Range     types.Range       `json:"range"`
	StartLine int               `json:"start_line"`
	EndLine   int               `json:"end_line"`
	Content   string            `json:"content"`
	Captures  map[string]string `json:"captures"`
}

// StructuralSearch searches for a pattern in a file or directory.
func StructuralSearch(ctx context.Context, rootPath, pattern, ext string) ([]StructuralMatch, error) {
	return StructuralSearchWithRule(ctx, rootPath, types.StructuralRule{Pattern: pattern}, ext)
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isAlphaNumeric(b byte) bool {
	return isAlpha(b) || (b >= '0' && b <= '9')
}

func preparePattern(parser *tree_sitter.Parser, pattern string, ext string) (*tree_sitter.Tree, *tree_sitter.Node, []byte) {
	// Pre-process pattern to make wildcards valid identifiers
	processedPattern := pattern
	processedPattern = strings.ReplaceAll(processedPattern, "$$$", "__SCT_MULTI__")
	
	// Replace $IDENTIFIER with __SCT_IDENTIFIER__
	var sb strings.Builder
	for i := 0; i < len(processedPattern); i++ {
		if processedPattern[i] == '$' && i+1 < len(processedPattern) && isAlpha(processedPattern[i+1]) {
			sb.WriteString("__SCT_")
			i++
			for i < len(processedPattern) && isAlphaNumeric(processedPattern[i]) {
				sb.WriteByte(processedPattern[i])
				i++
			}
			sb.WriteString("__")
			i-- 
		} else {
			sb.WriteByte(processedPattern[i])
		}
	}
	processedPattern = sb.String()

	patternTree := parser.Parse([]byte(processedPattern), nil)
	patternRoot := patternTree.RootNode()
	pContent := []byte(processedPattern)

	if patternRoot.HasError() {
		wrapped := processedPattern
		prefix := ""
		if ext == ".go" {
			prefix = "package main\nfunc _() {\n"
			wrapped = prefix + processedPattern + "\n}"
		} else if ext == ".ts" || ext == ".js" {
			prefix = "function _() {\n"
			wrapped = prefix + processedPattern + "\n}"
		}

		wTree := parser.Parse([]byte(wrapped), nil)
		if !wTree.RootNode().HasError() {
			pContent = []byte(wrapped)
			patternRoot = findTargetNode(wTree.RootNode(), pContent, processedPattern)
			patternTree.Close()
			patternTree = wTree
		} else {
			wTree.Close()
		}
	}

	// Drill down to the named node if it's a wrapper (like source_file)
	for patternRoot != nil && patternRoot.NamedChildCount() == 1 && patternRoot.ChildCount() > 0 {
		text := patternRoot.Utf8Text(pContent)
		if _, ok := isWildcard(text); ok {
			break
		}
		// Move to the first named child
		patternRoot = patternRoot.NamedChild(0)
	}

	return patternTree, patternRoot, pContent
}

// StructuralRefactor performs a structural search and replaces matches with a template.
func StructuralRefactor(ctx context.Context, filePath, pattern, template, ext string) (string, error) {
	cmd, err := utils.SafeCommand(ctx, "sg", "run", "--pattern", pattern, "--rewrite", template, filePath)
	if err != nil {
		return "", err
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// ast-grep returns 1 if no matches are found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}

		return "", fmt.Errorf("ast-grep failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

func findTargetNode(node *tree_sitter.Node, content []byte, pattern string) *tree_sitter.Node {
	patternTrim := strings.TrimSpace(pattern)
	
	// Recursively search in children first to find the most specific match
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		child := node.Child(i)
		res := findTargetNode(child, content, pattern)
		if res != nil {
			return res
		}
	}

	// Then check if the current node itself is the target
	nodeText := strings.TrimSpace(node.Utf8Text(content))
	if nodeText == patternTrim {
		kind := node.Kind()
		// If it's a wrapper like expression_statement, drill down to the actual expression
		if (kind == "expression_statement" || kind == "source_file" || kind == "program") && node.NamedChildCount() == 1 {
			return node.NamedChild(0)
		}
		
		// Priority nodes for structural matching
		if kind == "call_expression" || kind == "function_declaration" || kind == "assignment_expression" {
			return node
		}

		// Avoid returning root nodes or broad containers if possible
		if kind != "source_file" && kind != "function_declaration" && kind != "block" && kind != "program" {
			return node
		}
	}

	return nil
}

// StructuralSearchWithRule searches for a pattern in a file or directory using a StructuralRule.
func StructuralSearchWithRule(ctx context.Context, rootPath string, rule types.StructuralRule, ext string) ([]StructuralMatch, error) {
	ensureStructuralInit()

	lang, ok := languageConfigs[ext]
	if !ok {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		lang, ok = languageConfigs[ext]
		if !ok {
			return nil, nil
		}
	}

	patternParser := tree_sitter.NewParser()
	defer patternParser.Close()
	patternParser.SetLanguage(lang.Language)

	patternTree, patternRoot, pContent := preparePattern(patternParser, rule.Pattern, ext)
	defer patternTree.Close()

	if patternRoot == nil {
		return nil, nil
	}

	// Collect files to process
	var files []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Worker Pool implementation
	numWorkers := runtime.NumCPU()
	if len(files) < numWorkers {
		numWorkers = len(files)
	}
	if numWorkers == 0 {
		return nil, nil
	}

	jobs := make(chan string, len(files))
	results := make(chan []StructuralMatch, len(files))
	
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			
			// Fresh parser for every worker
			parser := tree_sitter.NewParser()
			defer parser.Close()
			parser.SetLanguage(lang.Language)

			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var fileMatches []StructuralMatch
			tree := parser.Parse(content, nil)
			if tree != nil {
				findMatches(tree.RootNode(), patternRoot, pContent, content, path, &fileMatches, rule, lang.Language, ext)
				tree.Close()
			}
			results <- fileMatches
			}
		}()
	}

	// Send jobs
	for _, path := range files {
		jobs <- path
	}
	close(jobs)

	// Result collector
	go func() {
		wg.Wait()
		close(results)
	}()

	var allMatches []StructuralMatch
	for res := range results {
		allMatches = append(allMatches, res...)
	}

	// Check if context was cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return allMatches, nil
}

func checkInside(node *tree_sitter.Node, insideKind string) bool {
	if insideKind == "" {
		return true
	}
	parent := node.Parent()
	for parent != nil {
		if strings.Contains(parent.Kind(), insideKind) {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

func checkHas(node *tree_sitter.Node, hasPattern string, content []byte, ext string, lang *tree_sitter.Language) bool {
	if hasPattern == "" {
		return true
	}
	
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)

	patternTree, patternRoot, pContent := preparePattern(parser, hasPattern, ext)
	if patternTree != nil {
		defer patternTree.Close()
	}

	if patternRoot == nil {
		return false
	}

	var found bool
	var traverse func(*tree_sitter.Node)
	traverse = func(n *tree_sitter.Node) {
		if found {
			return
		}
		
		tmpCaptures := make(map[string]string)
		if matchNodes(n, patternRoot, pContent, content, tmpCaptures) {
			found = true
			return
		}

		for i := uint(0); i < uint(n.ChildCount()); i++ {
			traverse(n.Child(i))
		}
	}
	
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		traverse(node.Child(i))
	}
	
	return found
}

func findMatches(node, pattern *tree_sitter.Node, patternContent, targetContent []byte, path string, matches *[]StructuralMatch, rule types.StructuralRule, lang *tree_sitter.Language, ext string) {
	captures := make(map[string]string)
	if matchNodes(node, pattern, patternContent, targetContent, captures) {
		if checkInside(node, rule.Inside) && checkHas(node, rule.Has, targetContent, ext, lang) {
			*matches = append(*matches, StructuralMatch{
				Path:      path,
				Range:     types.Range{Start: int(node.StartByte()), End: int(node.EndByte())},
				StartLine: int(node.StartPosition().Row) + 1,
				EndLine:   int(node.EndPosition().Row) + 1,
				Content:   node.Utf8Text(targetContent),
				Captures:  captures,
			})
		}
	}

	for i := uint(0); i < uint(node.ChildCount()); i++ {
		findMatches(node.Child(i), pattern, patternContent, targetContent, path, matches, rule, lang, ext)
	}
}

func isWildcard(text string) (string, bool) {
	if strings.HasPrefix(text, "__SCT_") {
		if text == "__SCT_MULTI__" {
			return "$$$", true
		}
		// Convert back to $V format for user-facing captures
		name := strings.TrimPrefix(text, "__SCT_")
		name = strings.TrimSuffix(name, "__")
		return "$" + name, true
	}
	return "", false
}

func matchNodes(node, pattern *tree_sitter.Node, patternContent, targetContent []byte, captures map[string]string) bool {
	pText := strings.TrimSpace(pattern.Utf8Text(patternContent))
	tText := strings.TrimSpace(node.Utf8Text(targetContent))

	if name, ok := isWildcard(pText); ok {
		captures[name] = tText
		return true
	}

	if node.Kind() != pattern.Kind() {
		return false
	}

	if pattern.ChildCount() == 0 {
		return tText == pText
	}

	pCount := uint(pattern.ChildCount())
	tCount := uint(node.ChildCount())
	pIdx := uint(0)
	tIdx := uint(0)

	for pIdx < pCount {
		pChild := pattern.Child(pIdx)
		pChildText := pChild.Utf8Text(patternContent)

		if name, ok := isWildcard(pChildText); ok && name == "$$$" {
			var multiContent strings.Builder

			// Look for the next pattern child that is not optional punctuation
			nextPIdx := pIdx + 1
			for nextPIdx < pCount && (pattern.Child(nextPIdx).Kind() == ";" || pattern.Child(nextPIdx).Kind() == ",") {
				nextPIdx++
			}

			// If it's the last meaningful pattern child, consume all remaining target children
			if nextPIdx == pCount {
				for tIdx < tCount {
					multiContent.WriteString(node.Child(tIdx).Utf8Text(targetContent))
					tIdx++
				}
				captures["$$$"] = multiContent.String()
				pIdx = pCount
				break
			}

			nextPChild := pattern.Child(nextPIdx)
			foundNext := false

			for tIdx < tCount {
				tmpCaptures := make(map[string]string)
				for k, v := range captures {
					tmpCaptures[k] = v
				}

				if matchNodes(node.Child(tIdx), nextPChild, patternContent, targetContent, tmpCaptures) {
					captures["$$$"] = multiContent.String()
					for k, v := range tmpCaptures {
						captures[k] = v
					}
					foundNext = true
					pIdx = nextPIdx + 1
					tIdx++
					break
				}

				multiContent.WriteString(node.Child(tIdx).Utf8Text(targetContent))
				tIdx++
			}

			if !foundNext {
				return false
			}
			continue
		}

		if tIdx >= tCount {
			if pChild.Kind() == ";" || pChild.Kind() == "," {
				pIdx++
				continue
			}
			return false
		}

		if matchNodes(node.Child(tIdx), pChild, patternContent, targetContent, captures) {
			pIdx++
			tIdx++
		} else if pChild.Kind() == ";" || pChild.Kind() == "," {
			pIdx++
		} else {
			return false
		}
	}

	return pIdx == pCount
}
