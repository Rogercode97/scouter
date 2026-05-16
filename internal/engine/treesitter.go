package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
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

	// Go Configuration: Native parser is preferred, TS is backup.
	goLang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	registerLanguage(".go", goLang,
		`(function_declaration name: (identifier) @name) @function 
         (method_declaration name: (field_identifier) @name) @method`,
		`(call_expression function: (identifier) @callee) (call_expression function: (selector_expression field: (field_identifier) @callee))`)

	// TS Configuration: Enriched to capture interface methods and MCP patterns
	tsLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	tsQuery := `(class_declaration name: (type_identifier) @name) @class 
         (function_declaration name: (identifier) @name) @function 
         (generator_function_declaration name: (identifier) @name) @function
         (variable_declarator name: (identifier) @name value: (arrow_function)) @function
         (method_definition name: (property_identifier) @name) @method 
         (interface_declaration name: (type_identifier) @name) @interface
         (interface_declaration name: (type_identifier) @iname body: (interface_body (method_signature (property_identifier) @mname))) @interface_spec
         (call_expression 
           function: (member_expression property: (property_identifier) @pname (#match? @pname "^(registerTool|registerResource|registerPrompt|tool|resource|prompt)$"))
           arguments: (arguments (string (string_fragment) @name))) @mcp_entry`
	tsCallQuery := `(call_expression function: (identifier) @callee) 
                    (call_expression function: (member_expression property: (property_identifier) @callee))
                    (call_expression function: (member_expression object: (identifier) @obj property: (property_identifier) @callee))`


	registerLanguage(".ts", tsLang, tsQuery, tsCallQuery)

	// TSX/JSX Configuration: uses LanguageTSX
	tsxLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX())
	registerLanguage(".tsx", tsxLang, tsQuery, tsCallQuery)
	registerLanguage(".jsx", tsxLang, tsQuery, tsCallQuery)
	registerLanguage(".js", tsLang, tsQuery, tsCallQuery)

	// Python Configuration: Class methods are the "interfaces" by convention
	pyLang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	registerLanguage(".py", pyLang,
		`(function_definition name: (identifier) @name) @function 
         (class_definition name: (identifier) @name) @class`,
		`(call function: (identifier) @callee) (call function: (attribute attribute: (identifier) @callee))`)
}

func registerLanguage(ext string, lang *tree_sitter.Language, qSrc, cSrc string) {
	q, err := tree_sitter.NewQuery(lang, qSrc)
	if err != nil {
		slog.Error("failed to register symbol query", "ext", ext, "error", err)
	}
	cq, err := tree_sitter.NewQuery(lang, cSrc)
	if err != nil {
		slog.Error("failed to register call query", "ext", ext, "error", err)
	}
	languageConfigs[ext] = &LanguageConfig{Language: lang, Query: q, CallQuery: cq}
}

func ParseWithTreeSitter(ctx context.Context, filePath string) ([]types.ASTPointer, []types.ASTCall, error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, err
	}

	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return []types.ASTPointer{}, []types.ASTCall{}, nil
	}

	if config.Query == nil || config.CallQuery == nil {
		return nil, nil, fmt.Errorf("tree-sitter queries not initialized for %s", ext)
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

	safeInt := func(u uint) int {
		i, err := utils.SafeUintToInt(u)
		if err != nil {
			return math.MaxInt
		}
		return i
	}

	var pointers []types.ASTPointer
	var calls []types.ASTCall

	// Query definitions
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(config.Query, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		var name, symType, recv, mname, iname string
		var symNode tree_sitter.Node
		for _, cap := range match.Captures {
			nameN := config.Query.CaptureNames()[cap.Index]
			switch nameN {
			case "name":
				name = cap.Node.Utf8Text(content)
			case "recv":
				recv = cap.Node.Utf8Text(content)
			case "mname":
				mname = cap.Node.Utf8Text(content)
			case "iname":
				iname = cap.Node.Utf8Text(content)
			default:
				symType = nameN
				symNode = cap.Node
			}
		}

		// Handle normal symbols
		if name != "" && symType != "interface_spec" {
			fullName := name
			if recv != "" {
				fullName = recv + "." + name
			}

			doc := extractDoc(symNode, content, ext)
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", symType, fullName)))
			pointers = append(pointers, types.ASTPointer{
				Type:           symType,
				Name:           fullName,
				Doc:            doc,
				Range:          types.Range{Start: safeInt(symNode.StartByte()), End: safeInt(symNode.EndByte())},
				StartLine:      safeInt(symNode.StartPosition().Row) + 1,
				EndLine:        safeInt(symNode.EndPosition().Row) + 1,
				Hash:           hex.EncodeToString(h[:]),
				StructuralHash: GetStructuralHash(&symNode, content),
			})
		}

		// Handle interface method specs
		if mname != "" {
			parentInterface := name
			if iname != "" {
				parentInterface = iname
			}
			fullMethodName := parentInterface + ":" + mname
			pointers = append(pointers, types.ASTPointer{
				Type:           "method_spec",
				Name:           fullMethodName,
				Range:          types.Range{Start: safeInt(symNode.StartByte()), End: safeInt(symNode.EndByte())},
				StartLine:      safeInt(symNode.StartPosition().Row) + 1,
				EndLine:        safeInt(symNode.EndPosition().Row) + 1,
				Hash:           utils.HashString(fmt.Sprintf("spec:%s", fullMethodName)),
				StructuralHash: GetStructuralHash(&symNode, content),
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
			calleePath := ""
			if callNode.Parent() != nil && callNode.Parent().Kind() == "call_expression" {
				if callNode.Kind() == "identifier" {
					calleePath = filePath
				}
			}

			calls = append(calls, types.ASTCall{
				CallerName: findTSCaller(callNode, content),
				CalleeName: callee,
				CalleePath: calleePath,
				LinkType:   "call",
				Path:       filePath,
				Line:       safeInt(callNode.StartPosition().Row) + 1,
			})
		}
	}

	return pointers, calls, nil
}

func findTSCaller(node tree_sitter_node, content []byte) string {
	curr := node.Parent()
	for curr != nil {
		kind := curr.Kind()
		if kind == "function_definition" || kind == "function_declaration" || kind == "method_definition" || kind == "method_declaration" {
			recvName := ""
			parentClass := curr.Parent()
			for parentClass != nil {
				if parentClass.Kind() == "class_declaration" || parentClass.Kind() == "class_definition" {
					if nameNode := parentClass.ChildByFieldName("name"); nameNode != nil {
						recvName = nameNode.Utf8Text(content)
						break
					}
				}
				parentClass = parentClass.Parent()
			}
			
			if name := curr.ChildByFieldName("name"); name != nil {
				methodName := name.Utf8Text(content)
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

type tree_sitter_node = tree_sitter.Node

func extractDoc(node tree_sitter.Node, content []byte, ext string) string {
	declNode := node
	for declNode.Kind() == "identifier" || declNode.Kind() == "property_identifier" || declNode.Kind() == "type_identifier" || declNode.Kind() == "field_identifier" {
		parent := declNode.Parent()
		if parent == nil {
			break
		}
		declNode = *parent
	}

	if ext == ".py" {
		block := declNode.ChildByFieldName("body")
		if block == nil {
			for i := uint32(0); i < uint32(declNode.ChildCount()); i++ { // #nosec G115 - ChildCount is safe to convert
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