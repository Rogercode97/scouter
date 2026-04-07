package engine

import (
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

// MaxSnippetSize is the limit for a surgical read (100KB)
const MaxSnippetSize = 100 * 1024

// MaxParseSize is the limit for indexing a file (2MB)
const MaxParseSize = 2 * 1024 * 1024

// ParseFile analyzes a file using the AST engine to index its structure.
func ParseFile(filePath string) ([]types.ASTPointer, error) {
	// 1. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	// 1.5. Size Limit Check (Indexing Memory Guard)
	fi, err := os.Stat(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("error stating file: %w", err)
	}
	if fi.Size() > MaxParseSize {
		return nil, fmt.Errorf("file too large to index (%d bytes), limit is %d bytes", fi.Size(), MaxParseSize)
	}

	// 2. Try native Go parser first (more reliable for now)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, validatedPath, nil, parser.ParseComments)
	if err == nil {
		var pointers []types.ASTPointer
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				pointers = append(pointers, types.ASTPointer{
					Type: "function",
					Name: fn.Name.Name,
					Range: types.Range{
						Start: startPos.Offset,
						End:   endPos.Offset,
					},
					StartLine: startPos.Line,
					EndLine:   endPos.Line,
					Hash:      "placeholder-hash",
				})
			}
			return true
		})
		if len(pointers) > 0 {
			return pointers, nil
		}
	}

	// 3. Try Tree-sitter for multi-language support as fallback
	pointers, err := ParseWithTreeSitter(validatedPath)
	if err == nil {
		return pointers, nil
	}

	// Fallback or specific error handling can go here
	return nil, fmt.Errorf("parsing failed for %s: %w", filePath, err)
}

// ReadSnippet reads a specific code snippet from a file using a byte range JSON string.
// Now includes: Sincronización, Encoding Guard, and Size Limits.
func ReadSnippet(filePath string, rangeJSON string) (string, error) {
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
	if requestedSize > MaxSnippetSize {
		return "", fmt.Errorf("requested snippet too large (%d bytes), limit is %d bytes", requestedSize, MaxSnippetSize)
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

	// 4. Encoding Guard (Ghost 3: Encoding Robustness)
	if !utf8.Valid(buffer) {
		return "", fmt.Errorf("binary or invalid UTF-8 data detected: Scouter only analyzes text-based source code")
	}

	return string(buffer), nil
}
