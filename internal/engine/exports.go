package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ParseExports parses re-export declarations from a file depending on its extension.
func ParseExports(ctx context.Context, filePath string) ([]types.ExportDeclaration, error) {
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
		return nil, fmt.Errorf("unsupported extension for exports: %s", ext)
	}

	parser := gotreesitter.NewParser(lang)
	tree, _ := parser.Parse(content)
	if tree == nil {
		return nil, nil
	}

	var exports []types.ExportDeclaration
	root := tree.RootNode()

	switch ext {
	case ".py":
		querySrc := `(assignment (identifier) @name (#eq? @name "__all__") (list (string (string_content) @export_item)))`
		q, err := gotreesitter.NewQuery(querySrc, lang)
		if err == nil {
			cursor := q.Exec(root, lang, content)
			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				for _, cap := range match.Captures {
					if cap.Name == "export_item" {
						exports = append(exports, types.ExportDeclaration{
							Symbol: string(cap.Node.Text(content)),
							File:   filePath,
							IsAll:  true,
						})
					}
				}
			}
		} else {
			fmt.Println("PY QUERY ERROR:", err)
		}

	case ".rs":
		querySrc := `
			(use_declaration 
				(visibility_modifier) 
				[
					(scoped_identifier (identifier) @export_item)
					(scoped_use_list (use_list (identifier) @export_item))
					(scoped_use_list (use_list (scoped_identifier (identifier) @export_item)))
					(identifier) @export_item
				]
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
					if cap.Name == "export_item" {
						exports = append(exports, types.ExportDeclaration{
							Symbol: string(cap.Node.Text(content)),
							File:   filePath,
						})
					}
				}
			}
		} else {
			fmt.Println("RS QUERY ERROR:", err)
		}

	case ".ts", ".tsx", ".js", ".jsx":
		querySrc := `
		(export_statement (string (string_fragment) @source)) @all_export
		(export_statement (export_clause (export_specifier (identifier) @name (identifier)? @alias)) (string (string_fragment) @source))
		`
		q, err := gotreesitter.NewQuery(querySrc, lang)
		if err == nil {
			cursor := q.Exec(root, lang, content)
			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				
				var source, name, alias string
				var isAll bool
				
				for _, cap := range match.Captures {
					if cap.Name == "source" {
						source = string(cap.Node.Text(content))
					} else if cap.Name == "name" {
						name = string(cap.Node.Text(content))
					} else if cap.Name == "alias" {
						alias = string(cap.Node.Text(content))
					} else if cap.Name == "all_export" {
						isAll = true
					}
				}
				
				if name != "" {
					sym := name
					if alias != "" && alias != name {
						sym = alias
					}
					exports = append(exports, types.ExportDeclaration{
						Symbol: sym,
						Source: source,
						File:   filePath,
					})
				} else if isAll && source != "" {
					exports = append(exports, types.ExportDeclaration{
						Symbol: "*",
						Source: source,
						File:   filePath,
						IsAll:  true,
					})
				}
			}
		} else {
			fmt.Println("TS QUERY ERROR:", err)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var deduped []types.ExportDeclaration
	for _, e := range exports {
		key := fmt.Sprintf("%s:%s:%v", e.Symbol, e.Source, e.IsAll)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, e)
		}
	}

	return deduped, nil
}
