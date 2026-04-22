package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// PointerResolver handles the resolution of MCP pointers to physical file ranges.
type PointerResolver struct {
	store store.Repository
}

func NewPointerResolver(st store.Repository) *PointerResolver {
	return &PointerResolver{store: st}
}

// Resolve converts a pointer (e.g., "main", a JSON range, or a 64-char hash) into a types.Range.
func (r *PointerResolver) Resolve(ctx context.Context, filePath, pointer string) (types.Range, error) {
	if pointer == "" {
		return types.Range{}, fmt.Errorf("empty pointer")
	}

	// 1. Fallback: try to parse the pointer as a JSON range if it looks like one.
	if strings.HasPrefix(pointer, "{") {
		var rng types.Range
		if err := json.Unmarshal([]byte(pointer), &rng); err == nil {
			return rng, nil
		}
	}

	// 2. Hash validation for full file (Solving the Pointer Paradox fully).
	if len(pointer) == 64 {
		currentFileHash, err := utils.CalculateHash(filePath)
		if err != nil {
			return types.Range{}, err
		}
		if currentFileHash == pointer {
			// It matches the full file hash. A range of 0, 0 in engine indicates full file read.
			return types.Range{Start: -1, End: -1}, nil
		}
	}

	// 3. Treat pointer as a symbol name and query the store.
	symbols, err := r.store.SearchSymbols(ctx, pointer, "")
	if err != nil {
		return types.Range{}, fmt.Errorf("failed to search for pointer: %w", err)
	}

	// 4. Filter symbols that match the exact filePath and name.
	for _, sym := range symbols {
		if sym.Path == filePath && sym.Name == pointer {
			return types.Range{
				Start: sym.StartByte,
				End:   sym.EndByte,
			}, nil
		}
	}

	return types.Range{}, fmt.Errorf("pointer '%s' not found in file '%s'", pointer, filePath)
}
