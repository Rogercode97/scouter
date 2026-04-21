package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
func ParseFile(ctx context.Context, filePath string, lspMgr *lsp.Manager) ([]types.ASTPointer, []types.ASTCall, error) {
	// 1. Context check
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	ext := filepath.Ext(filePath)
	if ext != ".go" {
		return ParseWithTreeSitter(ctx, filePath)
	}

	// 2. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, err
	}

	// 1.5. Size Limit Check (Indexing Memory Guard)
	fi, err := os.Stat(validatedPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error stating file: %w", err)
	}
	if fi.Size() > MaxParseSize {
		return nil, nil, fmt.Errorf("file too large to index (%d bytes), limit is %d bytes", fi.Size(), MaxParseSize)
	}

	// 2. Try native Go parser first (more reliable for now)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, validatedPath, nil, parser.ParseComments)
	if err == nil {
		var pointers []types.ASTPointer
		var calls []types.ASTCall

		type funcCtx struct {
			name           string
			end            token.Pos
			anonymousCount int
		}
		var funcStack []*funcCtx

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return false
			}

			// Pop functions from the stack when the current node is outside their scope.
			for len(funcStack) > 0 {
				top := funcStack[len(funcStack)-1]
				if n.Pos() >= top.end {
					funcStack = funcStack[:len(funcStack)-1]
				} else {
					break
				}
			}

			// Push a new function declaration onto the stack.
			if fn, ok := n.(*ast.FuncDecl); ok {
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				doc := utils.CleanComment(fn.Doc.Text())
				
				fullName := fn.Name.Name
				symType := "function"

				// TASK 1.1: Capture method receiver for Go Divine mapping
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

				content := fmt.Sprintf("%s:%s:%d:%d", symType, fullName, startPos.Offset, endPos.Offset)
				h := sha256.Sum256([]byte(content))
				pointers = append(pointers, types.ASTPointer{
					Type:      symType,
					Name:      fullName,
					Doc:       doc,
					Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
					StartLine: startPos.Line,
					EndLine:   endPos.Line,
					Hash:      hex.EncodeToString(h[:]),
				})
				funcStack = append(funcStack, &funcCtx{
					name: fullName,
					end:  fn.End(),
				})
			}

			// Capture Structs and Interfaces from GenDecl
			if gd, ok := n.(*ast.GenDecl); ok && (gd.Tok == token.STRUCT || gd.Tok == token.INTERFACE || gd.Tok == token.TYPE) {
				for _, spec := range gd.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						var symType string
						switch it := ts.Type.(type) {
						case *ast.StructType:
							symType = "class"
						case *ast.InterfaceType:
							symType = "interface"
							// TASK 2.1: Extract methods from interface for contract mapping
							if it.Methods != nil {
								for _, field := range it.Methods.List {
									if len(field.Names) > 0 {
										methodName := field.Names[0].Name
										fullMethodName := ts.Name.Name + ":" + methodName
										mStart := fset.Position(field.Pos())
										mEnd := fset.Position(field.End())
										pointers = append(pointers, types.ASTPointer{
											Type:      "method_spec",
											Name:      fullMethodName,
											Range:     types.Range{Start: mStart.Offset, End: mEnd.Offset},
											StartLine: mStart.Line,
											EndLine:   mEnd.Line,
											Hash:      utils.HashString(fmt.Sprintf("spec:%s", fullMethodName)),
										})
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
						pointers = append(pointers, types.ASTPointer{
							Type:      symType,
							Name:      ts.Name.Name,
							Doc:       doc,
							Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
							StartLine: startPos.Line,
							EndLine:   endPos.Line,
							Hash:      hex.EncodeToString(h[:]),
						})
					}
				}
			}

			// Push anonymous functions onto the stack.
			if fn, ok := n.(*ast.FuncLit); ok {
				parentName := "global"
				if len(funcStack) > 0 {
					top := funcStack[len(funcStack)-1]
					top.anonymousCount++
					parentName = top.name
					anonName := fmt.Sprintf("%s.func%d", parentName, top.anonymousCount)
					funcStack = append(funcStack, &funcCtx{
						name: anonName,
						end:  fn.End(),
					})
				} else {
					funcStack = append(funcStack, &funcCtx{
						name: "global.func",
						end:  fn.End(),
					})
				}
			}

			// If we are inside a function, record any call expressions.
			if call, ok := n.(*ast.CallExpr); ok {
				if len(funcStack) > 0 {
					caller := funcStack[len(funcStack)-1]
					calleeName, calleePath := resolveCallee(call.Fun, validatedPath)
					if calleeName != "" {
						calls = append(calls, types.ASTCall{
							CallerName: caller.name,
							CalleeName: calleeName,
							CalleePath: calleePath,
							LinkType:   "call",
							Path:       validatedPath,
							Line:       fset.Position(call.Lparen).Line,
						})
					}
				}
			}

			return true
		})

		return pointers, calls, nil
	}

	// 3. Try Tree-sitter for multi-language support as fallback
	pointers, calls, err := ParseWithTreeSitter(ctx, validatedPath)
	if err == nil {
		return pointers, calls, nil
	}

	return nil, nil, fmt.Errorf("parsing failed for %s: %w", filePath, err)
}

// resolveCallee attempts to get the name and potential path of the function being called.
func resolveCallee(fun ast.Expr, currentPath string) (string, string) {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name, currentPath
	case *ast.SelectorExpr:
		return f.Sel.Name, ""
	default:
		return "", ""
	}
}

// ReadFragment reads a specific code fragment and validates it against its expected hash.
func ReadFragment(ctx context.Context, filePath string, rangeJSON string, expectedHash string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return "", err
	}

	var r types.Range
	if err := json.Unmarshal([]byte(rangeJSON), &r); err != nil {
		return "", fmt.Errorf("error parsing range JSON: %w", err)
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

	if r.Start < 0 || int64(r.End) > fi.Size() || r.Start > r.End {
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

	if expectedHash != "" {
		currentFileHash, _ := utils.CalculateHash(validatedPath)
		if currentFileHash != "" && currentFileHash != expectedHash {
			return "", fmt.Errorf("integrity violation: the file has been modified since it was last indexed (expected %s, got %s); please re-index the project", expectedHash, currentFileHash)
		}
	}

	return string(buffer), nil
}