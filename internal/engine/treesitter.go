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
	Language    *tree_sitter.Language
	Query       *tree_sitter.Query
	SourceQuery string
}

var languageConfigs map[string]*LanguageConfig

func init() {
	languageConfigs = make(map[string]*LanguageConfig)

	// Go Configuration
	goLang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	goQuerySource := `
		(function_declaration (identifier) @name) @function
		(method_declaration (field_identifier) @name) @method
	`
	registerLanguage(".go", goLang, goQuerySource)

	// TypeScript / JavaScript Configuration
	tsLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	tsQuerySource := `
		(function_declaration (identifier) @name) @function
		(class_declaration (type_identifier) @name) @class
		(method_definition (property_identifier) @name) @method
	`
	registerLanguage(".ts", tsLang, tsQuerySource)
	registerLanguage(".tsx", tsLang, tsQuerySource)
	registerLanguage(".js", tsLang, tsQuerySource)
	registerLanguage(".jsx", tsLang, tsQuerySource)

	// Python Configuration
	pyLang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	pyQuerySource := `
		(function_definition name: (identifier) @name) @function
		(class_definition name: (identifier) @name) @class
	`
	registerLanguage(".py", pyLang, pyQuerySource)
}

func registerLanguage(ext string, lang *tree_sitter.Language, querySource string) {
	q, err := tree_sitter.NewQuery(lang, querySource)
	if err != nil {
		panic(fmt.Sprintf("HAKAISHIN CRITICAL: Failed to compile query for %s: %v\nQuery source:\n%s", ext, err, querySource))
	}

	languageConfigs[ext] = &LanguageConfig{
		Language:    lang,
		Query:       q,
		SourceQuery: querySource,
	}
}

func ParseWithTreeSitter(filePath string) ([]types.ASTPointer, error) {
	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	err = parser.SetLanguage(config.Language)
	if err != nil {
		return nil, fmt.Errorf("set language error: %w", err)
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("parser returned nil tree")
	}
	defer tree.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	// Use Matches instead of Captures for clearer grouping of symbol + name.
	matches := cursor.Matches(config.Query, tree.RootNode(), content)

	var pointers []types.ASTPointer
	for match := matches.Next(); match != nil; match = matches.Next() {
		var name string
		var symType string
		var symNode tree_sitter.Node
		var foundSym bool

		for _, capture := range match.Captures {
			captureName := config.Query.CaptureNames()[capture.Index]
			if captureName == "name" {
				name = capture.Node.Utf8Text(content)
			} else {
				symType = captureName
				symNode = capture.Node
				foundSym = true
			}
		}

		if name != "" && symType != "" && foundSym {
			startPos := symNode.StartPosition()
			endPos := symNode.EndPosition()

			pointers = append(pointers, types.ASTPointer{
				Type: symType,
				Name: name,
				Range: types.Range{
					Start: int(symNode.StartByte()),
					End:   int(symNode.EndByte()),
				},
				StartLine: int(startPos.Row) + 1,
				EndLine:   int(endPos.Row) + 1,
				Hash:      "placeholder-hash",
			})
		}
	}

	return pointers, nil
}
