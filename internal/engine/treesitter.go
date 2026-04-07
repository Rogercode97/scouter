package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/types"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type LanguageConfig struct {
	Language *tree_sitter.Language
	Query    string
}

func getLanguageConfig(ext string) (*LanguageConfig, error) {
	switch ext {
	case ".go":
		return &LanguageConfig{
			Language: tree_sitter.NewLanguage(tree_sitter_go.Language()),
			Query: `
				(function_declaration name: (identifier) @name) @function
				(method_declaration name: (identifier) @name) @method
			`,
		}, nil
	case ".ts", ".tsx", ".js", ".jsx":
		return &LanguageConfig{
			Language: tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()),
			Query: `
				(function_declaration name: (identifier) @name) @function
			`,
		}, nil
	case ".py":
		return &LanguageConfig{
			Language: tree_sitter.NewLanguage(tree_sitter_python.Language()),
			Query: `
				(function_definition name: (identifier) @name) @function
				(class_definition name: (identifier) @name) @class
			`,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}
}

func ParseWithTreeSitter(filePath string) ([]types.ASTPointer, error) {
	ext := filepath.Ext(filePath)
	config, err := getLanguageConfig(ext)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	err = parser.SetLanguage(config.Language)
	if err != nil {
		return nil, err
	}

	tree := parser.Parse(content, nil)
	defer tree.Close()

	query, err := tree_sitter.NewQuery(config.Language, config.Query)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captures := cursor.Captures(query, tree.RootNode(), content)

	var pointers []types.ASTPointer
	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := query.CaptureNames()[capture.Index]
		
		// We only care about the top-level captures like @function, @class, etc.
		// and they should have a child or be associated with a @name capture in the same match.
		if captureName == "name" {
			continue
		}

		// Find the name within this match
		var name string
		for _, c := range match.Captures {
			if query.CaptureNames()[c.Index] == "name" {
				name = c.Node.Utf8Text(content)
				break
			}
		}

		if name == "" {
			continue
		}

		startPos := capture.Node.StartPosition()
		endPos := capture.Node.EndPosition()

		pointers = append(pointers, types.ASTPointer{
			Type: captureName,
			Name: name,
			Range: types.Range{
				Start: int(capture.Node.StartByte()),
				End:   int(capture.Node.EndByte()),
			},
			StartLine: int(startPos.Row) + 1,
			EndLine:   int(endPos.Row) + 1,
			Hash:      "placeholder-hash",
		})
	}

	return pointers, nil
}
