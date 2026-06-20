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
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
)

var (
	structOnce sync.Once
)

func ensureStructuralInit() {
	structOnce.Do(func() {
	})
}

type StructuralMatch struct {
	Path      string            `json:"path"`
	Range     types.Range       `json:"range"`
	StartLine int               `json:"start_line"`
	EndLine   int               `json:"end_line"`
	Content   string            `json:"content"`
	Captures  map[string]string `json:"captures"`
}

func StructuralSearch(ctx context.Context, rootPath, pattern, ext string) ([]StructuralMatch, error) {
	return StructuralSearchWithRule(ctx, rootPath, types.StructuralRule{Pattern: pattern}, ext)
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isAlphaNumeric(b byte) bool {
	return isAlpha(b) || (b >= '0' && b <= '9')
}

func preparePattern(parser *gotreesitter.Parser, pattern string, ext string, lang *gotreesitter.Language) (*gotreesitter.Tree, *gotreesitter.Node, []byte) {
	processedPattern := pattern
	processedPattern = strings.ReplaceAll(processedPattern, "$$$", "__SCT_MULTI__")

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

	patternTree, _ := parser.Parse([]byte(processedPattern))
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

		wTree, _ := parser.Parse([]byte(wrapped))
		if wTree != nil {
			if !wTree.RootNode().HasError() {
				pContent = []byte(wrapped)
				patternRoot = findTargetNode(wTree.RootNode(), pContent, processedPattern, lang)
			}
		}
	}

	for patternRoot != nil && patternRoot.NamedChildCount() == 1 && patternRoot.ChildCount() > 0 {
		text := patternRoot.Text(pContent)
		if _, ok := isWildcard(text); ok {
			break
		}
		patternRoot = patternRoot.NamedChild(0)
	}

	return patternTree, patternRoot, pContent
}

func StructuralRefactor(ctx context.Context, filePath, pattern, template, ext string) (string, error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return "", err
	}
	cmd, err := utils.SafeCommand(ctx, "sg", "run", "--pattern", pattern, "--rewrite", template, filePath)
	if err != nil {
		return "", err
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if _, pathErr := exec.LookPath("sg"); pathErr != nil {
			return "", fmt.Errorf("ast-grep (sg) not found in PATH: %w", pathErr)
		}

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

func findTargetNode(node *gotreesitter.Node, content []byte, pattern string, lang *gotreesitter.Language) *gotreesitter.Node {
	patternTrim := strings.TrimSpace(pattern)

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		res := findTargetNode(child, content, pattern, lang)
		if res != nil {
			return res
		}
	}

	nodeText := strings.TrimSpace(node.Text(content))
	if nodeText == patternTrim {
		kind := node.Type(lang)
		if (kind == "expression_statement" || kind == "source_file" || kind == "program") && node.NamedChildCount() == 1 {
			return node.NamedChild(0)
		}

		if kind == "call_expression" || kind == "function_declaration" || kind == "assignment_expression" {
			return node
		}

		if kind != "source_file" && kind != "function_declaration" && kind != "block" && kind != "program" {
			return node
		}
	}

	return nil
}

func StructuralSearchWithRule(ctx context.Context, rootPath string, rule types.StructuralRule, ext string) ([]StructuralMatch, error) {
	rootPath, err := utils.ValidatePath(rootPath)
	if err != nil {
		return nil, err
	}
	ensureStructuralInit()

	langConfig, ok := languageConfigs[ext]
	if !ok {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		langConfig, ok = languageConfigs[ext]
		if !ok {
			return nil, nil
		}
	}

	patternParser := GetParser(langConfig.Language)
	defer PutParser(langConfig.Language, patternParser)

	patternTree, patternRoot, pContent := preparePattern(patternParser, rule.Pattern, ext, langConfig.Language)
	if patternTree != nil {
		defer patternTree.Release()
	}

	if patternRoot == nil {
		return nil, nil
	}

	var files []string
	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
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

			parser := GetParser(langConfig.Language)
			defer PutParser(langConfig.Language, parser)

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
				tree, _ := parser.Parse(content)
				if tree != nil {
					findMatches(tree.RootNode(), patternRoot, pContent, content, path, &fileMatches, rule, langConfig.Language, ext)
					tree.Release()
				}
				results <- fileMatches
			}
		}()
	}

	for _, path := range files {
		jobs <- path
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var allMatches []StructuralMatch
	for res := range results {
		allMatches = append(allMatches, res...)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return allMatches, nil
}

func checkInside(node *gotreesitter.Node, insideKind string, lang *gotreesitter.Language) bool {
	if insideKind == "" {
		return true
	}
	parent := node.Parent()
	for parent != nil {
		if strings.Contains(parent.Type(lang), insideKind) {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

func checkHas(node *gotreesitter.Node, hasPattern string, content []byte, ext string, lang *gotreesitter.Language) bool {
	if hasPattern == "" {
		return true
	}

	parser := GetParser(lang)
	defer PutParser(lang, parser)

	patternTree, patternRoot, pContent := preparePattern(parser, hasPattern, ext, lang)
	if patternTree != nil {
		defer patternTree.Release()
	}

	if patternRoot == nil {
		return false
	}

	var found bool
	var traverse func(*gotreesitter.Node)
	traverse = func(n *gotreesitter.Node) {
		if found {
			return
		}

		tmpCaptures := make(map[string]string)
		if matchNodes(n, patternRoot, pContent, content, tmpCaptures, lang) {
			found = true
			return
		}

		for i := 0; i < n.ChildCount(); i++ {
			traverse(n.Child(i))
		}
	}

	for i := 0; i < node.ChildCount(); i++ {
		traverse(node.Child(i))
	}

	return found
}

func findMatches(node, pattern *gotreesitter.Node, patternContent, targetContent []byte, path string, matches *[]StructuralMatch, rule types.StructuralRule, lang *gotreesitter.Language, ext string) {
	captures := make(map[string]string)
	if matchNodes(node, pattern, patternContent, targetContent, captures, lang) {
		if checkInside(node, rule.Inside, lang) && checkHas(node, rule.Has, targetContent, ext, lang) {
			start, _ := utils.SafeUintToInt(uint(node.StartByte()))
			end, _ := utils.SafeUintToInt(uint(node.EndByte()))
			sLine, _ := utils.SafeUintToInt(uint(node.StartPoint().Row))
			eLine, _ := utils.SafeUintToInt(uint(node.EndPoint().Row))

			*matches = append(*matches, StructuralMatch{
				Path:      path,
				Range:     types.Range{Start: start, End: end},
				StartLine: sLine + 1,
				EndLine:   eLine + 1,
				Content:   node.Text(targetContent),
				Captures:  captures,
			})
		}
	}

	for i := 0; i < node.ChildCount(); i++ {
		findMatches(node.Child(i), pattern, patternContent, targetContent, path, matches, rule, lang, ext)
	}
}

func isWildcard(text string) (string, bool) {
	if strings.HasPrefix(text, "__SCT_") {
		if text == "__SCT_MULTI__" {
			return "$$$", true
		}
		name := strings.TrimPrefix(text, "__SCT_")
		name = strings.TrimSuffix(name, "__")
		return "$" + name, true
	}
	return "", false
}

func matchNodes(node, pattern *gotreesitter.Node, patternContent, targetContent []byte, captures map[string]string, lang *gotreesitter.Language) bool {
	pText := strings.TrimSpace(pattern.Text(patternContent))
	tText := strings.TrimSpace(node.Text(targetContent))

	if name, ok := isWildcard(pText); ok {
		captures[name] = tText
		return true
	}

	if node.Type(lang) != pattern.Type(lang) {
		return false
	}

	if pattern.ChildCount() == 0 {
		return tText == pText
	}

	pCount := pattern.ChildCount()
	tCount := node.ChildCount()
	pIdx := 0
	tIdx := 0

	for pIdx < pCount {
		pChild := pattern.Child(pIdx)
		pChildText := pChild.Text(patternContent)

		if name, ok := isWildcard(pChildText); ok && name == "$$$" {
			var multiContent strings.Builder

			nextPIdx := pIdx + 1
			for nextPIdx < pCount && (pattern.Child(nextPIdx).Type(lang) == ";" || pattern.Child(nextPIdx).Type(lang) == ",") {
				nextPIdx++
			}

			if nextPIdx == pCount {
				for tIdx < tCount {
					multiContent.WriteString(node.Child(tIdx).Text(targetContent))
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

				if matchNodes(node.Child(tIdx), nextPChild, patternContent, targetContent, tmpCaptures, lang) {
					captures["$$$"] = multiContent.String()
					for k, v := range tmpCaptures {
						captures[k] = v
					}
					foundNext = true
					pIdx = nextPIdx + 1
					tIdx++
					break
				}

				multiContent.WriteString(node.Child(tIdx).Text(targetContent))
				tIdx++
			}

			if !foundNext {
				return false
			}
			continue
		}

		if tIdx >= tCount {
			if pChild.Type(lang) == ";" || pChild.Type(lang) == "," {
				pIdx++
				continue
			}
			return false
		}

		if matchNodes(node.Child(tIdx), pChild, patternContent, targetContent, captures, lang) {
			pIdx++
			tIdx++
		} else if pChild.Type(lang) == ";" || pChild.Type(lang) == "," {
			pIdx++
		} else {
			return false
		}
	}

	return pIdx == pCount
}
