package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// MaxSnippetSize is the limit for a surgical read (100KB)
const MaxSnippetSize = 100 * 1024

// ParseFile analyzes a file using the AST engine to index its structure.
func ParseFile(filePath string) ([]types.ASTPointer, error) {
	// 1. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	// 2. Try Tree-sitter for multi-language support
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

	content, err := os.ReadFile(validatedPath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// 3. Consistency Check (Ghost 2: File Consistency)
	// If the file length changed or offsets are invalid, something is wrong
	if r.Start < 0 || r.End > len(content) || r.Start > r.End {
		return "", fmt.Errorf("file out of sync or invalid range: index is stale, please re-index the file")
	}

	// 4. Encoding Guard (Ghost 3: Encoding Robustness)
	snippetBytes := content[r.Start:r.End]
	if !utf8.Valid(snippetBytes) {
		return "", fmt.Errorf("binary or invalid UTF-8 data detected: Scouter only analyzes text-based source code")
	}

	return string(snippetBytes), nil
}
