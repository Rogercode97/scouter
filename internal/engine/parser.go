package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"golang.org/x/tools/go/packages"
)

// MaxFragmentSize is the limit for a surgical read (100KB)
const MaxFragmentSize = 100 * 1024

// MaxParseSize is the limit for indexing a file (5MB)
const MaxParseSize = 5 * 1024 * 1024

// ParseFile analyzes a file using the AST engine to index its structure and call graph.
// It is now a wrapper around StreamSymbols for backward compatibility.
func ParseFile(ctx context.Context, filePath string, lspMgr *lsp.Manager) ([]types.ASTPointer, []types.ASTCall, []types.DataFlow, error) {
	itPointers, itCalls, itFlows, err := StreamSymbols(ctx, filePath)
	if err != nil {
		// Try Tree-sitter for multi-language support as fallback
		pIt, cIt, fIt, tsErr := StreamWithTreeSitter(ctx, filePath)
		if tsErr == nil {
			return slices.Collect(pIt), slices.Collect(cIt), slices.Collect(fIt), nil
		}
		return nil, nil, nil, fmt.Errorf("parsing failed for %s: %w (fallback error: %v)", filePath, err, tsErr)
	}
	return slices.Collect(itPointers), slices.Collect(itCalls), slices.Collect(itFlows), nil
}

// StreamSymbols analyzes a file and returns iterators for symbols, calls and data flows.
// Optimized for Go 1.25 to avoid large slice allocations.
func StreamSymbols(ctx context.Context, filePath string) (iter.Seq[types.ASTPointer], iter.Seq[types.ASTCall], iter.Seq[types.DataFlow], error) {
	// 1. Context check
	select {
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	default:
	}

	ext := filepath.Ext(filePath)
	if ext != ".go" {
		pIt, cIt, fIt, err := StreamWithTreeSitter(ctx, filePath)
		if err != nil {
			return nil, nil, nil, err
		}
		return pIt, cIt, fIt, nil
	}

	// 2. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, nil, err
	}

	// 1.5. Size Limit Check
	fi, err := os.Stat(validatedPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error stating file: %w", err)
	}
	if fi.Size() > MaxParseSize {
		return nil, nil, nil, fmt.Errorf("file too large to index (%d bytes), limit is %d bytes", fi.Size(), MaxParseSize)
	}

	file, fset, pkg, err := loadASTFile(validatedPath)
	if err != nil {
		return nil, nil, nil, err
	}
	pkgPath := pkg.PkgPath

	// For structural hashing consistency, we also parse with Tree-sitter for Go files
	var tsTree *gotreesitter.Tree
	var tsContent []byte
	tsContent, _ = os.ReadFile(validatedPath)
	tsParser := gotreesitter.NewParser(grammars.GoLanguage())
	tsTree, _ = tsParser.Parse(tsContent)

	// We return closures that perform the AST inspection lazily
	names := make(map[ast.Node]string)
	anonCounters := make(map[string]int)

	return func(yield func(types.ASTPointer) bool) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			stopped := false
			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				if stopped {
					return false
				}

				select {
				case <-ctx.Done():
					stopped = true
					return false
				default:
				}

				if n == nil {
					return true
				}

				if fn, ok := n.(*ast.FuncDecl); ok {
					startPos := fset.Position(fn.Pos())
					endPos := fset.Position(fn.End())
					identPos := fset.Position(fn.Name.Pos())
					doc := utils.CleanComment(fn.Doc.Text())
					fullName := fn.Name.Name
					symType := "function"
					receiverType := ""

					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						symType = "method"
						recvType, rType := extractMethodReceiver(fn)
						receiverType = rType
						if recvType != "" {
							fullName = recvType + "." + fn.Name.Name
						}
					}

					signature := extractSignature(fn.Type)
					content := fmt.Sprintf("%s:%s:%s:%d:%d", symType, fullName, signature, startPos.Offset, endPos.Offset)
					h := sha256.Sum256([]byte(content))

					var structuralHash string
					var metrics *types.SemanticMetrics
					if tsTree != nil {
						root := tsTree.RootNode()
						tsNode := root.NamedDescendantForByteRange(uint32(startPos.Offset), uint32(endPos.Offset))
						structuralHash = GetStructuralHash(tsNode, tsContent, grammars.GoLanguage())
						// Metrics are computed separately in treesitter.go for tsNode if used from there, but we compute them native if treesitter not used? No, actually treesitter_metrics is better. But let's compute Go metrics:
						metrics = computeGoMetrics(fn)
					} else {
						metrics = computeGoMetrics(fn)
					}

					p := types.ASTPointer{
						Type:           symType,
						Name:           fullName,
						PackagePath:    pkgPath,
						ReceiverType:   receiverType,
						Signature:      signature,
						Doc:            doc,
						Range:          types.Range{Start: startPos.Offset, End: endPos.Offset},
						StartLine:      identPos.Line,
						StartCol:       identPos.Column,
						EndLine:        endPos.Line,
						Hash:           hex.EncodeToString(h[:]),
						StructuralHash: structuralHash,
						Metrics:        metrics,
					}
					names[fn] = fullName
					if !yield(p) {
						stopped = true
						return false
					}
				}

				// Capture anonymous functions (closures/lambdas)
				if fl, ok := n.(*ast.FuncLit); ok {
					startPos := fset.Position(fl.Pos())
					endPos := fset.Position(fl.End())

					// Determine parent name for hierarchical synthetic naming
					parentName := "global"
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if name, ok := names[p]; ok {
							parentName = name
							break
						}
					}

					symType := "function"
					// Generate stable hierarchical name: Parent.funcN
					counterKey := pkgPath + ":" + parentName
					anonCounters[counterKey]++
					fullName := fmt.Sprintf("%s.func%d", parentName, anonCounters[counterKey])

					signature := extractSignature(fl.Type)
					content := fmt.Sprintf("%s:%s:%s:%d:%d", symType, fullName, signature, startPos.Offset, endPos.Offset)
					h := sha256.Sum256([]byte(content))

					var structuralHash string
					var metrics *types.SemanticMetrics
					if tsTree != nil {
						root := tsTree.RootNode()
						tsNode := root.NamedDescendantForByteRange(uint32(startPos.Offset), uint32(endPos.Offset))
						structuralHash = GetStructuralHash(tsNode, tsContent, grammars.GoLanguage())
					}
					metrics = computeGoMetrics(fl)

					p := types.ASTPointer{
						Type:           symType,
						Name:           fullName,
						PackagePath:    pkgPath,
						Signature:      signature,
						Range:          types.Range{Start: startPos.Offset, End: endPos.Offset},
						StartLine:      startPos.Line,
						StartCol:       startPos.Column,
						EndLine:        endPos.Line,
						Hash:           hex.EncodeToString(h[:]),
						StructuralHash: structuralHash,
						Metrics:        metrics,
					}
					names[fl] = fullName
					if !yield(p) {
						stopped = true
						return false
					}
				}


				// Capture Structs and Interfaces from GenDecl
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
					for _, spec := range gd.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok {
							identPos := fset.Position(ts.Name.Pos())
							var symType string
							switch it := ts.Type.(type) {
							case *ast.StructType:
								symType = "class"
							case *ast.InterfaceType:
								symType = "interface"
								if it.Methods != nil {
									for _, field := range it.Methods.List {
										if len(field.Names) > 0 {
											methodName := field.Names[0].Name
											fullMethodName := ts.Name.Name + "." + methodName
											mStart := fset.Position(field.Pos())
											mEnd := fset.Position(field.End())
											mIdent := fset.Position(field.Names[0].Pos())

											var sig string
											if ft, ok := field.Type.(*ast.FuncType); ok {
												sig = extractSignature(ft)
											}

											var structuralHash string
											if tsTree != nil {
												tsNode := tsTree.RootNode().NamedDescendantForByteRange(uint32(mStart.Offset), uint32(mEnd.Offset))
												structuralHash = GetStructuralHash(tsNode, tsContent, grammars.GoLanguage())
											}

											p := types.ASTPointer{
												Type:           "method_spec",
												Name:           fullMethodName,
												PackagePath:    pkgPath,
												Signature:      sig,
												Range:          types.Range{Start: mStart.Offset, End: mEnd.Offset},
												StartLine:      mIdent.Line,
												StartCol:       mIdent.Column,
												EndLine:        mEnd.Line,
												Hash:           utils.HashString(fmt.Sprintf("spec:%s:%s", fullMethodName, sig)),
												StructuralHash: structuralHash,
											}
											if !yield(p) {
												stopped = true
												return false
											}
										}
									}
								}
							default:
								continue
							}

							startPos := fset.Position(ts.Pos())
							endPos := fset.Position(ts.End())
							doc := utils.CleanComment(gd.Doc.Text())
							if doc == "" {
								doc = utils.CleanComment(ts.Doc.Text())
							}

							content := fmt.Sprintf("%s:%s:%d:%d", symType, ts.Name.Name, startPos.Offset, endPos.Offset)
							h := sha256.Sum256([]byte(content))

							var structuralHash string
							if tsTree != nil {
								tsNode := tsTree.RootNode().NamedDescendantForByteRange(uint32(startPos.Offset), uint32(endPos.Offset))
								structuralHash = GetStructuralHash(tsNode, tsContent, grammars.GoLanguage())
							}

							p := types.ASTPointer{
								Type:           symType,
								Name:           ts.Name.Name,
								PackagePath:    pkgPath,
								Doc:            doc,
								Range:          types.Range{Start: startPos.Offset, End: endPos.Offset},
								StartLine:      identPos.Line,
								StartCol:       identPos.Column,
								EndLine:        endPos.Line,
								Hash:           hex.EncodeToString(h[:]),
								StructuralHash: structuralHash,
							}
							if !yield(p) {
								stopped = true
								return false
							}
						}
					}
				}
				return true
			})
		}, func(yield func(types.ASTCall) bool) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Track types of parameters/variables in the current function scope
			scopeTypes := make(map[ast.Node]map[string]string)
			
			stopped := false

			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				if stopped {
					return false
				}

				select {
				case <-ctx.Done():
					stopped = true
					return false
				default:
				}

				if n == nil {
					return true
				}

				if fn, ok := n.(*ast.FuncDecl); ok {
					// Capture parameter types
					fScope := make(map[string]string)
					if fn.Type.Params != nil {
						for _, field := range fn.Type.Params.List {
							tName := exprToString(field.Type)
							// Normalize: remove pointers and package prefixes for better matching
							tName = strings.TrimPrefix(tName, "*")
							for _, name := range field.Names {
								fScope[name.Name] = tName
							}
						}
					}
					scopeTypes[fn] = fScope
				}

				if call, ok := n.(*ast.CallExpr); ok {
					var callerName string
					var currentScope map[string]string
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if name, ok := names[p]; ok {
							callerName = name
							currentScope = scopeTypes[p]
							break
						}
					}

					if callerName != "" {
						// Prepend pkgPath if it's not already qualified and not an anonymous function
						if !strings.Contains(callerName, "/") && !strings.HasPrefix(callerName, "global.func") && !strings.Contains(callerName, ".func") {
							callerName = pkgPath + "." + callerName
						} else if strings.Contains(callerName, ".func") && !strings.HasPrefix(callerName, pkgPath+".") {
							callerName = pkgPath + "." + callerName
						}

						calleeName, calleePath := resolveCallee(call.Fun, validatedPath)
						
						calleeName = resolveTypeInfoCallee(call, pkg, currentScope, pkgPath, calleeName)

						// If still a simple name or local selector, prepend pkgPath
						if calleeName != "" && !strings.Contains(calleeName, "/") && !strings.HasPrefix(calleeName, pkgPath+".") {
						  if !strings.Contains(calleeName, ".") {
						  calleeName = pkgPath + "." + calleeName
						  } else if parts := strings.Split(calleeName, "."); len(parts) == 2 {
						  // Check if it's a known type in scope
						  if tName, exists := currentScope[parts[0]]; exists {
						  calleeName = pkgPath + "." + tName + "." + parts[1]
						  } else {
						  calleeName = pkgPath + "." + calleeName
						  }
						  }
						}
						if calleeName != "" {
							c := types.ASTCall{
								CallerName: callerName,
								CalleeName: calleeName,
								CalleePath: calleePath,
								LinkType:   "call",
								Path:       validatedPath,
								Line:       fset.Position(call.Lparen).Line,
							}
							if !yield(c) {
								stopped = true
								return false
							}
						}
					}
				}
				return true
			})
		}, func(yield func(types.DataFlow) bool) {
			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}

				if call, ok := n.(*ast.CallExpr); ok {
					calleeName := exprToString(call.Fun)
					for i, arg := range call.Args {
						source := exprToString(arg)
						sink := fmt.Sprintf("%s:arg%d", calleeName, i)
						pos := fset.Position(arg.Pos())
						f := types.DataFlow{
							Source: source,
							Sink:   sink,
							Type:   "argument",
							Path:   validatedPath,
							Line:   pos.Line,
						}
						if !yield(f) {
							return false
						}
					}
				}

				if ret, ok := n.(*ast.ReturnStmt); ok {
					var currentFunc string
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if name, exists := names[p]; exists {
							currentFunc = name
							break
						}
					}
					if currentFunc != "" {
						for i, res := range ret.Results {
							source := exprToString(res)
							sink := fmt.Sprintf("%s:return%d", currentFunc, i)
							pos := fset.Position(res.Pos())
							f := types.DataFlow{
								Source: source,
								Sink:   sink,
								Type:   "return",
								Path:   validatedPath,
								Line:   pos.Line,
							}
							if !yield(f) {
								return false
							}
						}
					}
				}

				if as, ok := n.(*ast.AssignStmt); ok {
					isCallOnRhs := false
					var calleeName string
					if len(as.Rhs) == 1 {
						if call, ok := as.Rhs[0].(*ast.CallExpr); ok {
							isCallOnRhs = true
							calleeName = exprToString(call.Fun)
						}
					}

					for i, lhs := range as.Lhs {
						var source string
						if isCallOnRhs {
							source = fmt.Sprintf("%s:return%d", calleeName, i)
						} else {
							if i < len(as.Rhs) {
								source = exprToString(as.Rhs[i])
							} else if len(as.Rhs) == 1 {
								source = exprToString(as.Rhs[0])
							}
						}
						sink := exprToString(lhs)

						if source != "" && sink != "" && sink != "_" {
							pos := fset.Position(as.Pos())
							f := types.DataFlow{
								Source: source,
								Sink:   sink,
								Type:   "assignment",
								Path:   validatedPath,
								Line:   pos.Line,
							}
							if !yield(f) {
								return false
							}
						}
					}
				}

				if vs, ok := n.(*ast.ValueSpec); ok {
					isCallOnRhs := false
					var calleeName string
					if len(vs.Values) == 1 {
						if call, ok := vs.Values[0].(*ast.CallExpr); ok {
							isCallOnRhs = true
							calleeName = exprToString(call.Fun)
						}
					}

					for i, name := range vs.Names {
						var source string
						if isCallOnRhs {
							source = fmt.Sprintf("%s:return%d", calleeName, i)
						} else {
							if i < len(vs.Values) {
								source = exprToString(vs.Values[i])
							} else if len(vs.Values) == 1 {
								source = exprToString(vs.Values[0])
							}
						}
						sink := name.Name

						if source != "" && sink != "" && sink != "_" {
							pos := fset.Position(vs.Pos())
							f := types.DataFlow{
								Source: source,
								Sink:   sink,
								Type:   "assignment",
								Path:   validatedPath,
								Line:   pos.Line,
							}
							if !yield(f) {
								return false
							}
						}
					}
				}
				return true
			})
		}, nil
}
// resolveCallee attempts to get the name and potential path of the function being called.
func resolveCallee(fun ast.Expr, currentPath string) (string, string) {
	name := exprToString(fun)
	if name == "unknown" || name == "" {
		return "", ""
	}
	// Note: we don't return currentPath here because the store/linker 
	// handles global resolution within the project context.
	return name, ""
}

// ReadFragment reads a specific code fragment from a file within the given range.
func ReadFragment(ctx context.Context, filePath string, r types.Range) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return "", err
	}

	requestedSize := r.End - r.Start
	if requestedSize > MaxFragmentSize {
		return "", fmt.Errorf("requested fragment too large (%d bytes), limit is %d bytes", requestedSize, MaxFragmentSize)
	}

	f, err := os.Open(validatedPath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("error stating file: %w", err)
	}

	if r.Start < -1 || int64(r.End) > fi.Size() || (r.Start > r.End && r.Start != -1) {
		return "", fmt.Errorf("file out of sync or invalid range: index is stale, please re-index the file")
	}

	buffer := make([]byte, requestedSize)
	_, err = f.ReadAt(buffer, int64(r.Start))
	if err != nil {
		return "", fmt.Errorf("error reading range: %w", err)
	}

	if !utf8.Valid(buffer) {
		return "", fmt.Errorf("binary or invalid UTF-8 data detected: Scouter only analyzes text-based source code")
	}

	return string(buffer), nil
}

func extractSignature(ft *ast.FuncType) string {
	if ft == nil {
		return ""
	}

	params := ""
	if ft.Params != nil {
		var pList []string
		for _, field := range ft.Params.List {
			pType := exprToString(field.Type)
			if len(field.Names) > 0 {
				for range field.Names {
					pList = append(pList, pType)
				}
			} else {
				pList = append(pList, pType)
			}
		}
		params = "(" + strings.Join(pList, ", ") + ")"
	}

	results := ""
	if ft.Results != nil {
		var rList []string
		for _, field := range ft.Results.List {
			rList = append(rList, exprToString(field.Type))
		}
		results = "(" + strings.Join(rList, ", ") + ")"
	}

	if results == "" {
		return params
	}
	return params + " -> " + results
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.BasicLit:
		return t.Value
	case *ast.CallExpr:
		return exprToString(t.Fun) + "()"
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return fmt.Sprintf("interface{%d methods}", len(t.Methods.List))
	case *ast.StructType:
		if t.Fields == nil || len(t.Fields.List) == 0 {
			return "struct{}"
		}
		return fmt.Sprintf("struct{%d fields}", len(t.Fields.List))
	case *ast.ChanType:
		if t.Dir == ast.RECV {
			return "<-chan " + exprToString(t.Value)
		} else if t.Dir == ast.SEND {
			return "chan<- " + exprToString(t.Value)
		}
		return "chan " + exprToString(t.Value)
	case *ast.FuncType:
		return "func" + extractSignature(t)
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.ParenExpr:
		return "(" + exprToString(t.X) + ")"
	case *ast.BinaryExpr:
		return exprToString(t.X) + " " + t.Op.String() + " " + exprToString(t.Y)
	case *ast.UnaryExpr:
		return t.Op.String() + exprToString(t.X)
	default:
		return "unknown"
	}
}

func computeGoMetrics(node ast.Node) *types.SemanticMetrics {
	if node == nil {
		return nil
	}
	metrics := &types.SemanticMetrics{
		CyclomaticComplexity: 1,
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt:
			metrics.CyclomaticComplexity++
		case *ast.CaseClause:
			metrics.CyclomaticComplexity++
		case *ast.CommClause:
			metrics.CyclomaticComplexity++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				metrics.CyclomaticComplexity++
			}
			// check for err != nil
			if x.Op == token.NEQ {
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "err" {
					metrics.HasErrorHandling = true
				} else if id, ok := x.Y.(*ast.Ident); ok && id.Name == "err" {
					metrics.HasErrorHandling = true
				}
			}
		case *ast.GoStmt:
			metrics.IsAsync = true
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "panic" {
				metrics.HasExceptions = true
			}
		}
		return true
	})

	return metrics
}


func loadASTFile(validatedPath string) (*ast.File, *token.FileSet, *packages.Package, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: true,
		Dir:   filepath.Dir(validatedPath),
	}
	pkgs, err := packages.Load(cfg, "file="+validatedPath)
	if err != nil || len(pkgs) == 0 {
		pkgs, err = packages.Load(cfg, ".")
		if err != nil || len(pkgs) == 0 {
			return nil, nil, nil, fmt.Errorf("failed to load package: %v", err)
		}
	}

	var pkg *packages.Package
	var file *ast.File
	for _, p := range pkgs {
		for _, syntax := range p.Syntax {
			if p.Fset.Position(syntax.Pos()).Filename == validatedPath {
				pkg = p
				file = syntax
				break
			}
		}
		if pkg != nil {
			break
		}
	}

	if pkg == nil {
		if len(pkgs) == 1 && len(pkgs[0].Syntax) > 0 {
			pkg = pkgs[0]
			file = pkg.Syntax[0]
		} else {
			return nil, nil, nil, fmt.Errorf("file %s not found in loaded packages", validatedPath)
		}
	}
	return file, pkg.Fset, pkg, nil
}

func extractMethodReceiver(fn *ast.FuncDecl) (string, string) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", ""
	}
	recvType := ""
	receiverType := ""
	switch r := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		recvType = r.Name
		receiverType = "value"
	case *ast.StarExpr:
		receiverType = "pointer"
		if id, ok := r.X.(*ast.Ident); ok {
			recvType = id.Name
		} else if sel, ok := r.X.(*ast.SelectorExpr); ok {
			recvType = sel.Sel.Name
		}
	case *ast.SelectorExpr:
		recvType = r.Sel.Name
		receiverType = "value"
	}
	return recvType, receiverType
}

func resolveTypeInfoCallee(call *ast.CallExpr, pkg *packages.Package, currentScope map[string]string, pkgPath string, fallbackName string) string {
	calleeName := fallbackName
	if pkg.TypesInfo != nil {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if selection, ok := pkg.TypesInfo.Selections[sel]; ok {
				if obj := selection.Obj(); obj != nil {
					if objPkg := obj.Pkg(); objPkg != nil {
						if selection.Kind() == gotypes.MethodVal {
							if recv := selection.Recv(); recv != nil {
								typeName := ""
								if named, ok := recv.(*gotypes.Named); ok {
									typeName = named.Obj().Name()
								} else if ptr, ok := recv.(*gotypes.Pointer); ok {
									if named, ok := ptr.Elem().(*gotypes.Named); ok {
										typeName = named.Obj().Name()
									}
								}
								if typeName != "" {
									calleeName = objPkg.Path() + "." + typeName + "." + obj.Name()
								}
							}
						} else {
							calleeName = objPkg.Path() + "." + obj.Name()
						}
					}
				}
			} else if obj, ok := pkg.TypesInfo.Uses[sel.Sel]; ok {
				if objPkg := obj.Pkg(); objPkg != nil {
					calleeName = objPkg.Path() + "." + obj.Name()
				}
			}
		} else if ident, ok := call.Fun.(*ast.Ident); ok {
			if obj, ok := pkg.TypesInfo.Uses[ident]; ok {
				if objPkg := obj.Pkg(); objPkg != nil {
					calleeName = objPkg.Path() + "." + obj.Name()
				}
			}
		}
	}

	// Fallback to heuristic resolution if TypesInfo didn't give a full path
	if (!strings.Contains(calleeName, ".") || !strings.Contains(calleeName, "/")) && currentScope != nil {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok {
				if tName, exists := currentScope[x.Name]; exists {
					calleeName = tName + "." + sel.Sel.Name
					if !strings.Contains(calleeName, "/") {
						calleeName = pkgPath + "." + calleeName
					}
				}
			}
		}
	}
	return calleeName
}
