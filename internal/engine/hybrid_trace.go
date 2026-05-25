package engine

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ParseHybridTraces extracts structural dependencies (imports) and FFI declarations from a file.
func ParseHybridTraces(ctx context.Context, filePath string) ([]types.ASTCall, error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(filePath)
	var lang *gotreesitter.Language
	switch ext {
	case ".go":
		lang = grammars.GoLanguage()
	case ".py":
		lang = grammars.PythonLanguage()
	case ".rs":
		lang = grammars.RustLanguage()
	case ".ts", ".tsx", ".js", ".jsx":
		lang = grammars.TypescriptLanguage()
	default:
		return nil, nil // Not supported but no error
	}

	parser := gotreesitter.NewParser(lang)
	tree, _ := parser.Parse(content)
	if tree == nil {
		return nil, nil
	}

	var traces []types.ASTCall
	root := tree.RootNode()

	switch ext {
	case ".py":
		// Python: import module -> treat as caller = <file>, callee = module, LinkType = "import"
		// Query: (import_statement (dotted_name) @module)
		// Query: (import_from_statement module_name: (dotted_name) @module)
		querySrc := `
			(import_statement name: (dotted_name (identifier)) @module)
			(import_from_statement module_name: (dotted_name) @module name: (dotted_name) @symbol)
			(import_from_statement module_name: (identifier) @module name: (dotted_name) @symbol)
			(import_from_statement module_name: (identifier) @module name: (identifier) @symbol)
		`
		q, err := gotreesitter.NewQuery(querySrc, lang)
		if err == nil {
			cursor := q.Exec(root, lang, content)
			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				var moduleName string
				var symbolName string
				var line int
				for _, cap := range match.Captures {
					if cap.Name == "module" {
						moduleName = string(cap.Node.Text(content))
						line = int(cap.Node.StartPoint().Row + 1)
					}
					if cap.Name == "symbol" {
						symbolName = string(cap.Node.Text(content))
					}
				}
				if moduleName != "" {
					traces = append(traces, types.ASTCall{
						CallerName: filepath.Base(filePath), // file level caller
						CalleeName: moduleName,
						Line:       line,
						Path:       filePath,
						LinkType:   "import",
					})
					if symbolName != "" {
						traces = append(traces, types.ASTCall{
							CallerName: filepath.Base(filePath),
							CalleeName: symbolName,
							Line:       line,
							Path:       filePath,
							LinkType:   "import",
						})
					}
				}
			}
		}

	case ".rs":
		// Rust FFI: #[pyfunction]
		// Query: (function_item (attribute_item (attribute (identifier) @attr (#eq? @attr "pyfunction"))) name: (identifier) @func_name)
		querySrc := `
			(
				(attribute_item (attribute (identifier) @attr (#eq? @attr "pyfunction"))) 
				.
				(function_item name: (identifier) @func_name)
			)
		`
		q, err := gotreesitter.NewQuery(querySrc, lang)
		if err == nil {
			cursor := q.Exec(root, lang, content)
			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				for _, cap := range match.Captures {
					if cap.Name == "func_name" {
						// Rust exposes it, we map it as an FFI boundary link
						traces = append(traces, types.ASTCall{
							CallerName: string(cap.Node.Text(content)),
							CalleeName: string(cap.Node.Text(content)) + "_ffi", // logical FFI node
							Line:       int(cap.Node.StartPoint().Row + 1),
							Path:       filePath,
							LinkType:   "ffi",
						})
					}
				}
			}
		}
	}

	return traces, nil
}
