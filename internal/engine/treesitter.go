package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"iter"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type LanguageConfig struct {
	Language  *gotreesitter.Language
	Query     *gotreesitter.Query
	CallQuery *gotreesitter.Query
}

var languageConfigs map[string]*LanguageConfig

func init() {
	languageConfigs = make(map[string]*LanguageConfig)

	// Go Configuration
	goLang := grammars.GoLanguage()
	registerLanguage(".go", goLang,
		`(function_declaration name: (identifier) @name) @function 
         (method_declaration name: (field_identifier) @name) @method`,
		`(call_expression function: (identifier) @callee) (call_expression function: (selector_expression field: (field_identifier) @callee))`)

	// TS Configuration
	tsLang := grammars.TypescriptLanguage()
	tsQuery := `(class_declaration name: (type_identifier) @name) @class 
         (function_declaration name: (identifier) @name) @function 
         (generator_function_declaration name: (identifier) @name) @function
         (variable_declarator name: (identifier) @name value: (arrow_function)) @function
         (method_definition name: (property_identifier) @name) @method 
         (interface_declaration name: (type_identifier) @name) @interface
         (interface_declaration name: (type_identifier) @iname body: (interface_body (method_signature name: (property_identifier) @mname) @interface_spec))
         (call_expression 
           function: (member_expression property: (property_identifier) @pname (#match? @pname "^(registerTool|registerResource|registerPrompt|tool|resource|prompt)$"))
           arguments: (arguments (string (string_fragment) @name))) @mcp_entry`
	tsCallQuery := `(call_expression function: (identifier) @callee) 
                    (call_expression function: (member_expression property: (property_identifier) @callee))
                    (call_expression function: (member_expression object: (identifier) @obj property: (property_identifier) @callee))`

	registerLanguage(".ts", tsLang, tsQuery, tsCallQuery)

	// TSX/JSX Configuration
	tsxLang := grammars.TsxLanguage()
	registerLanguage(".tsx", tsxLang, tsQuery, tsCallQuery)
	registerLanguage(".jsx", tsxLang, tsQuery, tsCallQuery)
	registerLanguage(".js", tsLang, tsQuery, tsCallQuery)

	// Python Configuration
	pyLang := grammars.PythonLanguage()
	registerLanguage(".py", pyLang,
		`(function_definition name: (identifier) @name) @function 
         (class_definition name: (identifier) @name) @class
         (class_definition name: (identifier) @recv body: (block (function_definition name: (identifier) @name) @method))
         (class_definition name: (identifier) @recv body: (block (decorated_definition (function_definition name: (identifier) @name) @method)))`,
		`(call function: (identifier) @callee) (call function: (attribute attribute: (identifier) @callee))`)

	// Rust Configuration
	rustLang := grammars.RustLanguage()
	registerLanguage(".rs", rustLang,
		`(function_item name: (identifier) @name) @function
         (struct_item name: (type_identifier) @name) @class
         (trait_item name: (type_identifier) @name) @interface
         (impl_item type: (type_identifier) @recv body: (declaration_list (function_item name: (identifier) @name) @method))
         (trait_item name: (type_identifier) @iname body: (declaration_list (function_item name: (identifier) @mname) @interface_spec))`,
		`(call_expression function: (identifier) @callee)
         (call_expression function: (field_expression field: (field_identifier) @callee))
         (impl_item trait: (type_identifier) @trait_name type: (type_identifier) @type_name) @impl_block`)
}

func registerLanguage(ext string, lang *gotreesitter.Language, qSrc, cSrc string) {
	q, err := gotreesitter.NewQuery(qSrc, lang)
	if err != nil {
		slog.Error("failed to register symbol query", "ext", ext, "error", err)
	}
	cq, err := gotreesitter.NewQuery(cSrc, lang)
	if err != nil {
		slog.Error("failed to register call query", "ext", ext, "error", err)
	}
	languageConfigs[ext] = &LanguageConfig{Language: lang, Query: q, CallQuery: cq}
}

func StreamWithTreeSitter(ctx context.Context, filePath string) (iter.Seq[types.ASTPointer], iter.Seq[types.ASTCall], error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, err
	}

	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return func(yield func(types.ASTPointer) bool) {}, func(yield func(types.ASTCall) bool) {}, nil
	}

	if config.Query == nil || config.CallQuery == nil {
		return nil, nil, fmt.Errorf("tree-sitter queries not initialized for %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	safeInt := func(u uint32) int {
		i, err := utils.SafeUintToInt(uint(u))
		if err != nil {
			return math.MaxInt
		}
		return i
	}

	lang := config.Language
	parser := gotreesitter.NewParser(lang)
	tree, _ := parser.Parse(content)
	if tree == nil {
		return func(yield func(types.ASTPointer) bool) {}, func(yield func(types.ASTCall) bool) {}, nil
	}

	pointerIter := func(yield func(types.ASTPointer) bool) {
		cursor := config.Query.Exec(tree.RootNode(), lang, content)
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			var name, symType, recv, mname, iname string
			var symNode *gotreesitter.Node
			for _, cap := range match.Captures {
				nameN := cap.Name
				switch nameN {
				case "name":
					name = cap.Node.Text(content)
				case "recv":
					recv = cap.Node.Text(content)
				case "mname":
					mname = cap.Node.Text(content)
				case "iname":
					iname = cap.Node.Text(content)
				default:
					symType = nameN
					symNode = cap.Node
				}
			}

			// Handle normal symbols
			if name != "" && symType != "interface_spec" && symNode != nil {
				fullName := name
				if recv != "" {
					fullName = recv + "." + name
				}

				doc := extractDoc(symNode, content, ext, lang)
				h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", symType, fullName)))
				p := types.ASTPointer{
					Type:           symType,
					Name:           fullName,
					Doc:            doc,
					Range:          types.Range{Start: int(symNode.StartByte()), End: int(symNode.EndByte())},
					StartLine:      int(symNode.StartPoint().Row) + 1,
					EndLine:        int(symNode.EndPoint().Row) + 1,
					Hash:           hex.EncodeToString(h[:]),
					StructuralHash: GetStructuralHash(symNode, content, lang),
				}
				if !yield(p) {
					return
				}
			}

			// Handle interface method specs
			if mname != "" && symNode != nil {
				parentInterface := name
				if iname != "" {
					parentInterface = iname
				}
				fullMethodName := parentInterface + "." + mname
				p := types.ASTPointer{
					Type:           "interface_method",
					Name:           fullMethodName,
					Range:          types.Range{Start: int(symNode.StartByte()), End: int(symNode.EndByte())},
					StartLine:      int(symNode.StartPoint().Row) + 1,
					EndLine:        int(symNode.EndPoint().Row) + 1,
					Hash:           utils.HashString(fmt.Sprintf("spec:%s", fullMethodName)),
					StructuralHash: GetStructuralHash(symNode, content, lang),
				}
				if !yield(p) {
					return
				}
			}
		}
	}

	callIter := func(yield func(types.ASTCall) bool) {
		cursor := config.CallQuery.Exec(tree.RootNode(), lang, content)
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			var callee string
			var callNode *gotreesitter.Node
			var traitName, typeName string
			for _, cap := range match.Captures {
				name := cap.Name
				if name == "callee" {
					callee = cap.Node.Text(content)
					callNode = cap.Node
				} else if name == "trait_name" {
					traitName = cap.Node.Text(content)
				} else if name == "type_name" {
					typeName = cap.Node.Text(content)
					callNode = cap.Node
				}
			}

			if traitName != "" && typeName != "" && callNode != nil {
				c := types.ASTCall{
					CallerName: typeName,
					CalleeName: traitName,
					LinkType:   "implements",
					Path:       filePath,
					Line:       safeInt(callNode.StartPoint().Row) + 1,
				}
				if !yield(c) {
					return
				}
			} else if callee != "" && callNode != nil {
				calleePath := ""
				if callNode.Parent() != nil && callNode.Parent().Type(lang) == "call_expression" {
					if callNode.Type(lang) == "identifier" {
						calleePath = filePath
					}
				}

				c := types.ASTCall{
					CallerName: findTSCaller(callNode, content, lang),
					CalleeName: callee,
					CalleePath: calleePath,
					LinkType:   "call",
					Path:       filePath,
					Line:       safeInt(callNode.StartPoint().Row) + 1,
				}
				if !yield(c) {
					return
				}
			}
		}
	}

	return pointerIter, callIter, nil
}

func findTSCaller(node *gotreesitter.Node, content []byte, lang *gotreesitter.Language) string {
	curr := node.Parent()
	for curr != nil {
		kind := curr.Type(lang)
		if kind == "function_definition" || kind == "function_declaration" || kind == "method_definition" || kind == "method_declaration" || kind == "function_item" {
			recvName := ""
			parentClass := curr.Parent()
			for parentClass != nil {
				if parentClass.Type(lang) == "class_declaration" || parentClass.Type(lang) == "class_definition" {
					if nameNode := parentClass.ChildByFieldName("name", lang); nameNode != nil {
						recvName = nameNode.Text(content)
						break
					}
				} else if parentClass.Type(lang) == "impl_item" {
					if typeNode := parentClass.ChildByFieldName("type", lang); typeNode != nil {
						recvName = typeNode.Text(content)
						break
					}
				}
				parentClass = parentClass.Parent()
			}

			if name := curr.ChildByFieldName("name", lang); name != nil {
				methodName := name.Text(content)
				if recvName != "" {
					return recvName + "." + methodName
				}
				return methodName
			}
		}
		curr = curr.Parent()
	}
	return "global"
}

func extractDoc(node *gotreesitter.Node, content []byte, ext string, lang *gotreesitter.Language) string {
	declNode := node
	for declNode.Type(lang) == "identifier" || declNode.Type(lang) == "property_identifier" || declNode.Type(lang) == "type_identifier" || declNode.Type(lang) == "field_identifier" {
		parent := declNode.Parent()
		if parent == nil {
			break
		}
		declNode = parent
	}

	if ext == ".py" {
		block := declNode.ChildByFieldName("body", lang)
		if block == nil {
			for i := uint32(0); i < uint32(declNode.ChildCount()); i++ {
				child := declNode.Child(int(i))

				if child.Type(lang) == "block" {
					block = child
					break
				}
			}
		}

		if block != nil && block.ChildCount() > 0 {
			first := block.Child(0)
			if first.Type(lang) == "string" {
				return utils.CleanComment(first.Text(content))
			} else if first.Type(lang) == "expression_statement" && first.ChildCount() > 0 {
				expr := first.Child(0)
				if expr.Type(lang) == "string" {
					return utils.CleanComment(expr.Text(content))
				}
			}
		}
	}

	var comments []string
	curr := declNode.PrevSibling()
	for curr != nil {
		kind := curr.Type(lang)
		if kind == "comment" || kind == "line_comment" || kind == "block_comment" {
			comments = append([]string{curr.Text(content)}, comments...)
			curr = curr.PrevSibling()
		} else if strings.TrimSpace(curr.Text(content)) == "" {
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