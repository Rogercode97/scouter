package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type LanguageConfig struct {
	Language  *tree_sitter.Language
	Query     *tree_sitter.Query
	CallQuery *tree_sitter.Query
}

var languageConfigs map[string]*LanguageConfig

func init() {
	languageConfigs = make(map[string]*LanguageConfig)

	// Go Configuration
	goLang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	registerLanguage(".go", goLang,
		`(function_declaration (identifier) @name) @function (method_declaration (field_identifier) @name) @method`,
		`(call_expression function: (identifier) @callee) (call_expression function: (selector_expression field: (field_identifier) @callee))`)

	// TS Configuration
	tsLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	registerLanguage(".ts", tsLang,
		`(class_declaration name: (type_identifier) @name) @class (function_declaration name: (identifier) @name) @function (method_definition name: (property_identifier) @name) @method (interface_declaration name: (type_identifier) @name) @interface`,
		`(call_expression function: (identifier) @callee) (call_expression function: (member_expression property: (property_identifier) @callee))`)

	// Python Configuration
	pyLang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	registerLanguage(".py", pyLang,
		`(function_definition name: (identifier) @name) @function (class_definition name: (identifier) @name) @class`,
		`(call function: (identifier) @callee) (call function: (attribute attribute: (identifier) @callee))`)
}

func registerLanguage(ext string, lang *tree_sitter.Language, qSrc, cSrc string) {
	q, _ := tree_sitter.NewQuery(lang, qSrc)
	cq, _ := tree_sitter.NewQuery(lang, cSrc)
	languageConfigs[ext] = &LanguageConfig{Language: lang, Query: q, CallQuery: cq}
}

func ParseWithTreeSitter(ctx context.Context, filePath string) ([]types.ASTPointer, []types.ASTCall, error) {
	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported: %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(config.Language)

	tree := parser.Parse(content, nil)
	defer tree.Close()

	var pointers []types.ASTPointer
	var calls []types.ASTCall

	// Query definitions
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(config.Query, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		var name, symType string
		var symNode tree_sitter.Node
		for _, cap := range match.Captures {
			nameN := config.Query.CaptureNames()[cap.Index]
			if nameN == "name" {
				name = cap.Node.Utf8Text(content)
			} else {
				symType = nameN
				symNode = cap.Node
			}
		}
		if name != "" {
			doc := extractDoc(symNode, content, ext)
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", symType, name)))
			startPos := symNode.StartPosition()
			endPos := symNode.EndPosition()
			pointers = append(pointers, types.ASTPointer{
				Type:      symType,
				Name:      name,
				Doc:       doc,
				Range:     types.Range{Start: int(symNode.StartByte()), End: int(symNode.EndByte())},
				StartLine: int(startPos.Row) + 1,
				EndLine:   int(endPos.Row) + 1,
				Hash:      hex.EncodeToString(h[:]),
			})
		}
	}

	// Query calls
	callMatches := cursor.Matches(config.CallQuery, tree.RootNode(), content)
	for match := callMatches.Next(); match != nil; match = callMatches.Next() {
		var callee string
		var callNode tree_sitter.Node
		for _, cap := range match.Captures {
			if config.CallQuery.CaptureNames()[cap.Index] == "callee" {
				callee = cap.Node.Utf8Text(content)
				callNode = cap.Node
			}
		}
		if callee != "" {
			calls = append(calls, types.ASTCall{
				CallerName: findCaller(callNode, content),
				CalleeName: callee,
				Path:       filePath,
				Line:       int(callNode.StartPosition().Row) + 1,
			})
		}
	}

	return pointers, calls, nil
}

func findCaller(node tree_sitter.Node, content []byte) string {
	curr := node.Parent()
	for curr != nil {
		kind := curr.Kind()
		if kind == "function_definition" || kind == "function_declaration" || kind == "method_definition" {
			if name := curr.ChildByFieldName("name"); name != nil {
				return name.Utf8Text(content)
			}
		}
		curr = curr.Parent()
	}
	return "global"
}

func extractDoc(node tree_sitter.Node, content []byte, ext string) string {
	declNode := node
	for declNode.Kind() == "identifier" || declNode.Kind() == "property_identifier" || declNode.Kind() == "type_identifier" || declNode.Kind() == "field_identifier" {
		parent := declNode.Parent()
		if parent == nil {
			break
		}
		declNode = *parent
	}

	// 1. Python Docstrings
	if ext == ".py" {
		block := declNode.ChildByFieldName("body")
		if block == nil {
			for i := uint32(0); i < uint32(declNode.ChildCount()); i++ {
				child := declNode.Child(uint(i))
				if child.Kind() == "block" {
					block = child
					break
				}
			}
		}

		if block != nil && block.ChildCount() > 0 {
			first := block.Child(0)
			if first.Kind() == "expression_statement" && first.ChildCount() > 0 {
				expr := first.Child(0)
				if expr.Kind() == "string" {
					return utils.CleanComment(expr.Utf8Text(content))
				}
			}
		}
	}

	// 2. Backward Sibling Traversal
	var comments []string
	curr := declNode.PrevSibling()
	for curr != nil {
		kind := curr.Kind()
		if kind == "comment" || kind == "line_comment" || kind == "block_comment" {
			comments = append([]string{curr.Utf8Text(content)}, comments...)
			curr = curr.PrevSibling()
		} else if strings.TrimSpace(curr.Utf8Text(content)) == "" {
			curr = curr.PrevSibling()
		} else {
			break
		}
	}

	if len(comments) > 0 {
		return utils.CleanComment(strings.Join(comments, "\n"))
	}

	return ""
}
