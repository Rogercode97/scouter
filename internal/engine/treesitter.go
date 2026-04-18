package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/types"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
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

	// TypeScript Configuration
	tsLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	tsQuerySource := `
		(class_declaration name: (type_identifier) @name) @class
		(function_declaration name: (identifier) @name) @function
		(method_definition name: (property_identifier) @name) @method
		(interface_declaration name: (type_identifier) @name) @interface
		(variable_declarator name: (identifier) @name value: (arrow_function)) @function
	`
	registerLanguage(".ts", tsLang, tsQuerySource)
	registerLanguage(".tsx", tsLang, tsQuerySource)

	// JavaScript Configuration
	jsLang := tree_sitter.NewLanguage(tree_sitter_javascript.Language())
	jsQuerySource := `
		(class_declaration name: (identifier) @name) @class
		(function_declaration name: (identifier) @name) @function
		(method_definition name: (property_identifier) @name) @method
		(variable_declarator name: (identifier) @name value: (arrow_function)) @function
	`
	registerLanguage(".js", jsLang, jsQuerySource)
	registerLanguage(".jsx", jsLang, jsQuerySource)

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
		log.Printf("HAKAISHIN WARNING: Failed to compile query for %s: %v. Language registration skipped.\n", ext, err)
		return
	}

	languageConfigs[ext] = &LanguageConfig{
		Language:    lang,
		Query:       q,
		SourceQuery: querySource,
	}
}

func ParseWithTreeSitter(ctx context.Context, filePath string) ([]types.ASTPointer, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat error: %w", err)
	}
	if info.Size() > 5*1024*1024 {
		return nil, fmt.Errorf("file too large for AST indexing (>5MB): %s", filePath)
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
		// Check context in every iteration for long files
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

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

			// Generate content hash to satisfy 64-char validation (Divine Fix)
			contentHash := fmt.Sprintf("%s:%s:%d:%d", symType, name, symNode.StartByte(), symNode.EndByte())
			h := sha256.Sum256([]byte(contentHash))

			pointers = append(pointers, types.ASTPointer{
				Type: symType,
				Name: name,
				Range: types.Range{
					Start: int(symNode.StartByte()),
					End:   int(symNode.EndByte()),
				},
				StartLine: int(startPos.Row) + 1,
				EndLine:   int(endPos.Row) + 1,
				Hash:      hex.EncodeToString(h[:]),
			})
		}
	}

	return pointers, nil
}
