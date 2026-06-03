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
	"sync"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type LanguageConfig struct {
	Language    *gotreesitter.Language
	Query       *gotreesitter.Query
	CallQuery   *gotreesitter.Query
	ImportQuery *gotreesitter.Query
	FlowQuery   *gotreesitter.Query
}

var languageConfigs map[string]*LanguageConfig

func init() {
	languageConfigs = make(map[string]*LanguageConfig)

	// Go Configuration
	goLang := grammars.GoLanguage()
	registerLanguage(".go", goLang,
		`(function_declaration name: (identifier) @name) @function 
         (method_declaration name: (field_identifier) @name) @method
         (func_literal) @function`,
		`(call_expression function: [(identifier) (selector_expression)] @callee)`,
		`(import_spec path: [(interpreted_string_literal) (raw_string_literal)] @import)`,
		"")

	// TS Configuration
	tsLang := grammars.TypescriptLanguage()
	tsQuery := `(class_declaration name: (type_identifier) @name) @class 
         (function_declaration name: (identifier) @name) @function 
         (generator_function_declaration name: (identifier) @name) @function
         (variable_declarator name: (identifier) @name value: [(arrow_function) (function_expression)]) @function
         (method_definition name: (property_identifier) @name) @method 
         (public_field_definition name: (property_identifier) @name value: [(arrow_function) (function_expression)]) @method
         (pair key: (property_identifier) @name value: [(arrow_function) (function_expression)]) @method
         (arrow_function) @function
         (function_expression) @function
         (interface_declaration name: (type_identifier) @name) @interface
         (interface_declaration name: (type_identifier) @iname body: (interface_body (method_signature name: (property_identifier) @mname) @interface_spec))
         (call_expression 
           function: (member_expression property: (property_identifier) @pname (#match? @pname "^(registerTool|registerResource|registerPrompt|tool|resource|prompt)$"))
           arguments: (arguments (string (string_fragment) @name))) @mcp_entry`
	tsCallQuery := `(call_expression function: [
                      (identifier) @callee
                      (member_expression property: (property_identifier) @callee)
                      (member_expression object: (member_expression property: (property_identifier))) @callee
                    ])`
	tsImportQuery := `(import_statement source: (string) @import)`
	tsFlowQuery := `(variable_declarator name: (identifier) @sink value: (_) @source)
                   (assignment_expression left: (identifier) @sink right: (_) @source)
                   (call_expression function: [(identifier) (member_expression)] @call arguments: (arguments (_) @argument))
                   (return_statement (_) @return)
                   (arrow_function body: (_) @return)`

	registerLanguage(".ts", tsLang, tsQuery, tsCallQuery, tsImportQuery, tsFlowQuery)

	// TSX/JSX Configuration
	tsxLang := grammars.TsxLanguage()
	registerLanguage(".tsx", tsxLang, tsQuery, tsCallQuery, tsImportQuery, tsFlowQuery)
	registerLanguage(".jsx", tsxLang, tsQuery, tsCallQuery, tsImportQuery, tsFlowQuery)
	registerLanguage(".js", tsLang, tsQuery, tsCallQuery, tsImportQuery, tsFlowQuery)

	// Python Configuration
	pyLang := grammars.PythonLanguage()
	pyFlowQuery := `(assignment left: (identifier) @sink right: (_) @source)
                   (call function: [(identifier) (attribute)] @call arguments: (argument_list (_) @argument))
                   (return_statement (_) @return)`
	registerLanguage(".py", pyLang,
		`(function_definition name: (identifier) @name) @function 
         (class_definition name: (identifier) @name) @class
         (class_definition name: (identifier) @recv body: (block (function_definition name: (identifier) @name) @method))
         (class_definition name: (identifier) @recv body: (block (decorated_definition (function_definition name: (identifier) @name) @method)))
         (class_definition name: (identifier) @recv body: (block (expression_statement (assignment left: (identifier) @name right: (lambda))))) @method
         (lambda) @function`,
		`(call function: [(identifier) (attribute)] @callee)`,
		`(import_statement name: (dotted_name) @import) (import_from_statement module_name: (dotted_name) @import)`,
		pyFlowQuery)

	// Rust Configuration
	rustLang := grammars.RustLanguage()
	rustFlowQuery := `(let_declaration pattern: (identifier) @sink value: (_) @source)
                     (assignment_expression left: (identifier) @sink right: (_) @source)
                     (call_expression function: [(identifier) (field_expression)] @call arguments: (arguments (_) @argument))
                     (return_expression (_) @return)`
	registerLanguage(".rs", rustLang,
		`(function_item name: (identifier) @name) @function
         (struct_item name: (type_identifier) @name) @class
         (trait_item name: (type_identifier) @name) @interface
         (impl_item type: (type_identifier) @recv body: (declaration_list (function_item name: (identifier) @name) @method))
         (trait_item name: (type_identifier) @iname body: (declaration_list (function_item name: (identifier) @mname) @interface_spec))
         (closure_expression) @function`,
		`(call_expression function: [(identifier) (field_expression)] @callee)
         (impl_item trait: (type_identifier) @trait_name type: (type_identifier) @type_name) @impl_block`,
		`(use_declaration argument: (_) @import)`,
		rustFlowQuery)
}

func registerLanguage(ext string, lang *gotreesitter.Language, qSrc, cSrc, iSrc, fSrc string) {
	q, err := gotreesitter.NewQuery(qSrc, lang)
	if err != nil {
		slog.Error("failed to register symbol query", "ext", ext, "error", err)
	}
	cq, err := gotreesitter.NewQuery(cSrc, lang)
	if err != nil {
		slog.Error("failed to register call query", "ext", ext, "error", err)
	}
	iq, err := gotreesitter.NewQuery(iSrc, lang)
	if err != nil {
		slog.Error("failed to register import query", "ext", ext, "error", err)
	}
	var fq *gotreesitter.Query
	if fSrc != "" {
		fq, err = gotreesitter.NewQuery(fSrc, lang)
		if err != nil {
			slog.Error("failed to register flow query", "ext", ext, "error", err)
		}
	}
	languageConfigs[ext] = &LanguageConfig{Language: lang, Query: q, CallQuery: cq, ImportQuery: iq, FlowQuery: fq}
}

func StreamWithTreeSitter(ctx context.Context, filePath string) (iter.Seq[types.ASTPointer], iter.Seq[types.ASTCall], iter.Seq[types.DataFlow], error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, nil, err
	}

	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return func(yield func(types.ASTPointer) bool) {}, func(yield func(types.ASTCall) bool) {}, func(yield func(types.DataFlow) bool) {}, nil
	}

	if config.Query == nil || config.CallQuery == nil {
		return nil, nil, nil, fmt.Errorf("tree-sitter queries not initialized for %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, nil, err
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
		return func(yield func(types.ASTPointer) bool) {}, func(yield func(types.ASTCall) bool) {}, func(yield func(types.DataFlow) bool) {}, nil
	}

	var (
		sharedSymbolNames map[uint32]string
		symbolsOnce       sync.Once
	)

	getSharedSymbolNames := func() map[uint32]string {
		symbolsOnce.Do(func() {
			sharedSymbolNames = make(map[uint32]string)
			anonCounters := make(map[string]int)
			pCursor := config.Query.Exec(tree.RootNode(), lang, content)
			for {
				match, ok := pCursor.NextMatch()
				if !ok {
					break
				}
				var name, symType, recv string
				var symNode *gotreesitter.Node
				for _, cap := range match.Captures {
					nameN := cap.Name
					switch nameN {
					case "name":
						name = cap.Node.Text(content)
					case "recv":
						recv = cap.Node.Text(content)
					case "mname", "iname":
						// skip
					default:
						symType = nameN
						symNode = cap.Node
					}
				}

				parentName := "global"
				if symNode != nil {
					curr := symNode.Parent()
					for curr != nil {
						if n, ok := sharedSymbolNames[curr.EndByte()]; ok {
							parentName = n
							break
						}
						curr = curr.Parent()
					}
				}

				if symType == "function" && name == "" && symNode != nil {
					p := parentName
					if p == "" {
						p = "global"
					}
					anonCounters[p]++
					name = fmt.Sprintf("func%d", anonCounters[p])
				}

				if name != "" && symType != "interface_spec" && symNode != nil {
					fullName := name
					if recv != "" {
						fullName = recv + "." + name
					} else if parentName != "" && (parentName != "global" || strings.HasPrefix(name, "func")) {
						fullName = parentName + "." + name
					}
					sharedSymbolNames[symNode.EndByte()] = fullName
				}
			}
		})
		return sharedSymbolNames
	}

	pointerIter := func(yield func(types.ASTPointer) bool) {
		symbolNames := make(map[uint32]string)
		anonCounters := make(map[string]int)

		pCursor := config.Query.Exec(tree.RootNode(), lang, content)
		for {
			match, ok := pCursor.NextMatch()
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

			// Calculate hierarchical parent name
			parentName := "global"
			if symNode != nil {
				curr := symNode.Parent()
				for curr != nil {
					if n, ok := symbolNames[curr.EndByte()]; ok {
						parentName = n
						break
					}
					curr = curr.Parent()
				}
			}

			// Capture anonymous functions (closures/lambdas)
			if symType == "function" && name == "" && symNode != nil {
				p := parentName
				if p == "" {
					p = "global"
				}
				anonCounters[p]++
				name = fmt.Sprintf("func%d", anonCounters[p])
			}

			// Handle normal symbols
			if name != "" && symType != "interface_spec" && symNode != nil {
				fullName := name
				if recv != "" {
					fullName = recv + "." + name
				} else if parentName != "" && (parentName != "global" || strings.HasPrefix(name, "func")) {
					fullName = parentName + "." + name
				}

				symbolNames[symNode.EndByte()] = fullName

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
					LogicHash:      GetLogicHash(symNode, content, lang),
					Metrics:        computeSemanticMetrics(symNode, lang, content),
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
					LogicHash:      GetLogicHash(symNode, content, lang),
					Metrics:        computeSemanticMetrics(symNode, lang, content),
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
					CallerName: findTSCaller(callNode, content, lang, getSharedSymbolNames()),
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

	flowIter := func(yield func(types.DataFlow) bool) {
		if config.FlowQuery == nil {
			return
		}
		cursor := config.FlowQuery.Exec(tree.RootNode(), lang, content)
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

			var source, sink, call string
			var node *gotreesitter.Node
			var argNode, retNode *gotreesitter.Node
			for _, cap := range match.Captures {
				switch cap.Name {
				case "source":
					source = cap.Node.Text(content)
				case "sink":
					sink = cap.Node.Text(content)
					node = cap.Node
				case "call":
					call = cap.Node.Text(content)
				case "argument":
					source = cap.Node.Text(content)
					argNode = cap.Node
				case "return":
					source = cap.Node.Text(content)
					retNode = cap.Node
				}
			}

			if source != "" && sink != "" && node != nil {
				f := types.DataFlow{
					Source: source,
					Sink:   sink,
					Type:   "assignment",
					Path:   filePath,
					Line:   int(node.StartPoint().Row) + 1,
				}
				if !yield(f) {
					return
				}
			}

			if call != "" && argNode != nil {
				// Determine argument position
				pos := 0
				parent := argNode.Parent()
				if parent != nil {
					for i := 0; i < int(parent.NamedChildCount()); i++ {
						if parent.NamedChild(i).EndByte() == argNode.EndByte() {
							pos = i
							break
						}
					}
				}
				f := types.DataFlow{
					Source: source,
					Sink:   fmt.Sprintf("%s:arg%d", call, pos),
					Type:   "argument",
					Path:   filePath,
					Line:   int(argNode.StartPoint().Row) + 1,
				}
				if !yield(f) {
					return
				}
			}

			if retNode != nil {
				currentFunc := findTSCaller(retNode, content, lang, getSharedSymbolNames())
				f := types.DataFlow{
					Source: source,
					Sink:   fmt.Sprintf("%s:return0", currentFunc),
					Type:   "return",
					Path:   filePath,
					Line:   int(retNode.StartPoint().Row) + 1,
				}
				if !yield(f) {
					return
				}
			}
		}
	}

	return pointerIter, callIter, flowIter, nil
}

func findTSCaller(node *gotreesitter.Node, content []byte, lang *gotreesitter.Language, symbolNames map[uint32]string) string {
	curr := node.Parent()
	for curr != nil {
		if name, ok := symbolNames[curr.EndByte()]; ok {
			return name
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

func computeSemanticMetrics(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte) *types.SemanticMetrics {
	metrics := &types.SemanticMetrics{
		CyclomaticComplexity: 1, // Base complexity is 1
	}

	if node == nil {
		return metrics
	}

	metrics.CognitiveComplexity = computeCognitiveComplexity(node, lang, content)

	var traverse func(n *gotreesitter.Node)
	traverse = func(n *gotreesitter.Node) {
		if n == nil {
			return
		}

		kind := n.Type(lang)

		// Cyclomatic Complexity factors
		switch kind {
		case "if_statement", "for_statement", "while_statement", "catch_clause", "except_clause",
			"match_arm", "case", "case_statement", "case_clause", "communication_case":
			metrics.CyclomaticComplexity++
		case "binary_expression", "boolean_operator":
			operator := n.ChildByFieldName("operator", lang)
			if operator != nil {
				opText := string(operator.Text(content))
				if opText == "&&" || opText == "||" || opText == "and" || opText == "or" {
					metrics.CyclomaticComplexity++
				}
			}
		}

		// Async
		if kind == "async" || kind == "await_expression" || kind == "go_statement" {
			metrics.IsAsync = true
		}

		// Error handling
		if kind == "try_statement" || kind == "except_clause" || kind == "catch_clause" {
			metrics.HasErrorHandling = true
		}
		if kind == "binary_expression" {
			op := n.ChildByFieldName("operator", lang)
			if op != nil && string(op.Text(content)) == "!=" {
				left := n.ChildByFieldName("left", lang)
				right := n.ChildByFieldName("right", lang)
				if left != nil && right != nil {
					lt := string(left.Text(content))
					rt := string(right.Text(content))
					if (lt == "err" && rt == "nil") || (lt == "nil" && rt == "err") {
						metrics.HasErrorHandling = true
					}
				}
			}
		}

		// Exceptions
		if kind == "throw_statement" || kind == "raise_statement" || kind == "panic" || kind == "panic_statement" {
			metrics.HasExceptions = true
		}
		if kind == "call_expression" {
			callee := n.ChildByFieldName("function", lang)
			if callee != nil && string(callee.Text(content)) == "panic" {
				metrics.HasExceptions = true
			}
		}

		for i := 0; i < n.ChildCount(); i++ {
			traverse(n.Child(i))
		}
	}

	traverse(node)
	return metrics
}

func computeCognitiveComplexity(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte) int {
	var traverse func(n *gotreesitter.Node, nesting int) int
	traverse = func(n *gotreesitter.Node, nesting int) int {
		if n == nil {
			return 0
		}
		kind := n.Type(lang)
		score := 0
		isNestingStructure := false

		switch kind {
		case "if_statement", "for_statement", "while_statement", "catch_clause", "except_clause", "conditional_expression":
			score += 1 + (nesting * nesting)
			isNestingStructure = true
		case "switch_statement":
			score += 1
			isNestingStructure = true
		case "goto_statement", "labeled_statement":
			score += 1 + nesting
		case "func_literal", "arrow_function", "lambda":
			isNestingStructure = true
		case "break_statement", "continue_statement":
			if n.ChildCount() > 1 {
				score += 1
			}
		case "binary_expression", "boolean_operator":
			operator := n.ChildByFieldName("operator", lang)
			if operator != nil {
				opText := string(operator.Text(content))
				if opText == "&&" || opText == "||" || opText == "and" || opText == "or" {
					score += 1
				}
			}
		}

		nextNesting := nesting
		if isNestingStructure {
			nextNesting++
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			score += traverse(n.Child(i), nextNesting)
		}
		return score
	}
	return traverse(node, 0)
}

func ExtractImports(ctx context.Context, filePath string) ([]string, error) {
	filePath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return nil, nil
	}

	if config.ImportQuery == nil {
		return nil, fmt.Errorf("import query not initialized for %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lang := config.Language
	parser := gotreesitter.NewParser(lang)
	tree, _ := parser.Parse(content)
	if tree == nil {
		return nil, nil
	}

	var imports []string
	cursor := config.ImportQuery.Exec(tree.RootNode(), lang, content)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		for _, cap := range match.Captures {
			if cap.Name == "import" {
				text := cap.Node.Text(content)
				text = strings.Trim(text, "\"'`")
				imports = append(imports, text)
			}
		}
	}

	seen := make(map[string]bool)
	var uniqueImports []string
	for _, imp := range imports {
		if !seen[imp] {
			seen[imp] = true
			uniqueImports = append(uniqueImports, imp)
		}
	}

	return uniqueImports, nil
}
