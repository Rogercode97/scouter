package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// MaxFragmentSize is the limit for a surgical read (100KB)
const MaxFragmentSize = 100 * 1024

// MaxParseSize is the limit for indexing a file (5MB)
const MaxParseSize = 5 * 1024 * 1024

// ParseFile analyzes a file using the AST engine to index its structure and call graph.
// It is now a wrapper around StreamSymbols for backward compatibility.
func ParseFile(ctx context.Context, filePath string, lspMgr *lsp.Manager) ([]types.ASTPointer, []types.ASTCall, error) {
	itPointers, itCalls, err := StreamSymbols(ctx, filePath)
	if err != nil {
		// Try Tree-sitter for multi-language support as fallback
		p, c, tsErr := ParseWithTreeSitter(ctx, filePath)
		if tsErr == nil {
			return p, c, nil
		}
		return nil, nil, fmt.Errorf("parsing failed for %s: %w (fallback error: %v)", filePath, err, tsErr)
	}
	return slices.Collect(itPointers), slices.Collect(itCalls), nil
}

// StreamSymbols analyzes a file and returns iterators for symbols and calls.
// Optimized for Go 1.25 to avoid large slice allocations.
func StreamSymbols(ctx context.Context, filePath string) (iter.Seq[types.ASTPointer], iter.Seq[types.ASTCall], error) {
	// 1. Context check
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	ext := filepath.Ext(filePath)
	if ext != ".go" {
		// Tree-sitter fallback still returns slices for now, we wrap them in iterators
		p, c, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			return nil, nil, err
		}
		return slices.Values(p), slices.Values(c), nil
	}

	// 2. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, err
	}

	// 1.5. Size Limit Check
	fi, err := os.Stat(validatedPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error stating file: %w", err)
	}
	if fi.Size() > MaxParseSize {
		return nil, nil, fmt.Errorf("file too large to index (%d bytes), limit is %d bytes", fi.Size(), MaxParseSize)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, validatedPath, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	// We return closures that perform the AST inspection lazily
	return func(yield func(types.ASTPointer) bool) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				select {
				case <-ctx.Done():
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
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						symType = "method"
						recvType := ""
						switch r := fn.Recv.List[0].Type.(type) {
						case *ast.Ident:
							recvType = r.Name
						case *ast.StarExpr:
							if id, ok := r.X.(*ast.Ident); ok {
								recvType = id.Name
							}
						}
						if recvType != "" {
							fullName = recvType + "." + fn.Name.Name
						}
					}

					signature := extractSignature(fn.Type)
					content := fmt.Sprintf("%s:%s:%s:%d:%d", symType, fullName, signature, startPos.Offset, endPos.Offset)
					h := sha256.Sum256([]byte(content))
					p := types.ASTPointer{
						Type:      symType,
						Name:      fullName,
						Signature: signature,
						Doc:       doc,
						Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
						StartLine: identPos.Line,
						StartCol:  identPos.Column,
						EndLine:   endPos.Line,
						Hash:      hex.EncodeToString(h[:]),
					}
					if !yield(p) {
						return false
					}
				}

				// Capture Structs and Interfaces from GenDecl
				if gd, ok := n.(*ast.GenDecl); ok && (gd.Tok == token.STRUCT || gd.Tok == token.INTERFACE || gd.Tok == token.TYPE) {
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
											fullMethodName := ts.Name.Name + ":" + methodName
											mStart := fset.Position(field.Pos())
											mEnd := fset.Position(field.End())
											mIdent := fset.Position(field.Names[0].Pos())

											var sig string
											if ft, ok := field.Type.(*ast.FuncType); ok {
												sig = extractSignature(ft)
											}

											p := types.ASTPointer{
												Type:      "method_spec",
												Name:      fullMethodName,
												Signature: sig,
												Range:     types.Range{Start: mStart.Offset, End: mEnd.Offset},
												StartLine: mIdent.Line,
												StartCol:  mIdent.Column,
												EndLine:   mEnd.Line,
												Hash:      utils.HashString(fmt.Sprintf("spec:%s:%s", fullMethodName, sig)),
											}
											if !yield(p) {
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
							p := types.ASTPointer{
								Type:      symType,
								Name:      ts.Name.Name,
								Doc:       doc,
								Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
								StartLine: identPos.Line,
								StartCol:  identPos.Column,
								EndLine:   endPos.Line,
								Hash:      hex.EncodeToString(h[:]),
							}
							if !yield(p) {
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

			names := make(map[ast.Node]string)
			anonCounts := make(map[ast.Node]int)
			globalAnonCount := 0

			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}

				if n == nil {
					return true
				}

				if fn, ok := n.(*ast.FuncDecl); ok {
					names[fn] = fn.Name.Name
				} else if fn, ok := n.(*ast.FuncLit); ok {
					parentName := "global"
					var parentNode ast.Node
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if _, ok := p.(*ast.FuncDecl); ok {
							parentNode = p
							parentName = names[p]
							break
						}
						if _, ok := p.(*ast.FuncLit); ok {
							parentNode = p
							parentName = names[p]
							break
						}
					}
					var count int
					if parentNode != nil {
						anonCounts[parentNode]++
						count = anonCounts[parentNode]
					} else {
						globalAnonCount++
						count = globalAnonCount
					}
					anonName := fmt.Sprintf("%s.func%d", parentName, count)
					names[fn] = anonName
				}

				if call, ok := n.(*ast.CallExpr); ok {
					var callerName string
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if name, ok := names[p]; ok {
							callerName = name
							break
						}
					}

					if callerName != "" {
						calleeName, calleePath := resolveCallee(call.Fun, validatedPath)
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
	switch f := fun.(type) {
	case *ast.Ident:
		// Local call in the same file
		return f.Name, currentPath
	case *ast.SelectorExpr:
		// Potential call to another package or a method on a struct
		if x, ok := f.X.(*ast.Ident); ok {
			// Heuristic: if X is lowercase, it might be a variable (method call).
			// If X is Uppercase, it might be a package name.
			// For now, we return the selector string.
			return x.Name + "." + f.Sel.Name, ""
		}
		return f.Sel.Name, ""
	default:
		return "", ""
	}
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
	default:
		return "unknown"
	}
}