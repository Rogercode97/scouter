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
	"unicode/utf8"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// MaxFragmentSize is the limit for a surgical read (100KB)
const MaxFragmentSize = 100 * 1024

// MaxParseSize is the limit for indexing a file (5MB)
const MaxParseSize = 5 * 1024 * 1024

// ParseFile analyzes a file using the AST engine to index its structure and call graph.
func ParseFile(ctx context.Context, filePath string) ([]types.ASTPointer, []types.ASTCall, error) {
	// 1. Context check
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
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
		var funcStack []*ast.FuncDecl // Stack to manage (nested) function contexts

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return false
			}

			// Pop functions from the stack when the current node is outside their scope.
			// This handles leaving a function's body.
			for len(funcStack) > 0 {
				topFunc := funcStack[len(funcStack)-1]
				if n.Pos() >= topFunc.End() {
					funcStack = funcStack[:len(funcStack)-1]
				} else {
					break
				}
			}

			// Push a new function declaration onto the stack.
			if fn, ok := n.(*ast.FuncDecl); ok {
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				content := fmt.Sprintf("%s:%s:%d:%d", "function", fn.Name.Name, startPos.Offset, endPos.Offset)
				h := sha256.Sum256([]byte(content))
				pointers = append(pointers, types.ASTPointer{
					Type:      "function",
					Name:      fn.Name.Name,
					Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
					StartLine: startPos.Line,
					EndLine:   endPos.Line,
					Hash:      hex.EncodeToString(h[:]),
				})
				funcStack = append(funcStack, fn)
			}

			// If we are inside a function, record any call expressions.
			if call, ok := n.(*ast.CallExpr); ok {
				if len(funcStack) > 0 {
					caller := funcStack[len(funcStack)-1]
					calleeName := extractCalleeName(call.Fun)
					if calleeName != "" {
						calls = append(calls, types.ASTCall{
							CallerName: caller.Name.Name,
							CalleeName: calleeName,
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

	// Fallback or specific error handling can go here
	return nil, nil, fmt.Errorf("parsing failed for %s: %w", filePath, err)
}

// extractCalleeName attempts to get the name of the function being called.
func extractCalleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	default:
		return "" // Could be a more complex expression, ignore for now
	}
}

// ReadFragment reads a specific code fragment and validates it against its expected hash.
func ReadFragment(ctx context.Context, filePath string, rangeJSON string, expectedHash string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 1. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return "", err
	}

	var r types.Range
	if err := json.Unmarshal([]byte(rangeJSON), &r); err != nil {
		return "", fmt.Errorf("error parsing range JSON: %w", err)
	}

	// 2. Size Limit Check (Ghost 4: MCP UX)
	requestedSize := r.End - r.Start
	if requestedSize > MaxFragmentSize {
		return "", fmt.Errorf("requested fragment too large (%d bytes), limit is %d bytes", requestedSize, MaxFragmentSize)
	}

	// 3. Open file and read only the requested range (File Seeking)
	f, err := os.Open(validatedPath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	// Check file size for consistency
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

	// 4. Encoding Guard
	if !utf8.Valid(buffer) {
		return "", fmt.Errorf("binary or invalid UTF-8 data detected: Scouter only analyzes text-based source code")
	}

	// 5. Hash Validation (Divine Integrity)
	if expectedHash != "" {
		currentFileHash, _ := utils.CalculateHash(validatedPath)
		if currentFileHash != "" && currentFileHash != expectedHash {
			return "", fmt.Errorf("integrity violation: the file has been modified since it was last indexed (expected %s, got %s); please re-index the project", expectedHash, currentFileHash)
		}
	}

	return string(buffer), nil
}
