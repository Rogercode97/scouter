package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

// LinkInterfaces performs semantic linking between interfaces and their implementations
// using the LSP 'textDocument/implementation' request.
func LinkInterfaces(ctx context.Context, repo store.Repository, lspMgr *lsp.Manager) error {
	interfaces, err := repo.GetInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		client, err := lspMgr.GetClient(ctx, iface.Path)
		if err != nil {
			// Skip if LSP client is not available for this file type
			continue
		}

		// Prepare implementation params. LSP uses 0-based lines and columns.
		// We use 1-based lines and columns in our store (from tree-sitter).
		
		charPos := 0
		if iface.StartCol > 0 {
			charPos = iface.StartCol - 1
		}

		params := lsp.ImplementationParams{
			TextDocumentPositionParams: lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: "file://" + iface.Path,
				},
				Position: lsp.Position{
					Line:      iface.StartLine - 1,
					Character: charPos,
				},
			},
		}

		locations, err := client.Implementation(ctx, params)
		if err != nil {
			fmt.Printf("  [Linker] LSP error for %s: %v\n", iface.Name, err)
			continue
		}

		fmt.Printf("  [Linker] Found %d implementations for %s\n", len(locations), iface.Name)

		for _, loc := range locations {
			destPath := strings.TrimPrefix(loc.URI, "file://")
			
			// Only save if the destination is not the same as the interface itself
			if destPath == iface.Path && loc.Range.Start.Line == iface.StartLine-1 {
				continue
			}

			// We don't have the implementation name easily from LSP Location,
			// but we can save it as an "implements" link to the file/line.
			// Scouter V2.0 will resolve this semantically during impact analysis.
			call := store.Call{
				CallerName: "impl", // Generic name for implementation
				CalleeName: iface.Name,
				CalleePath: iface.Path,
				LinkType:   "implements",
				Path:       destPath,
				Line:       loc.Range.Start.Line + 1, // Back to 1-based
			}

			if err := repo.SaveCall(ctx, call); err != nil {
				fmt.Printf("  [Linker] SaveCall error for %s: %v\n", iface.Name, err)
				continue
			}
		}
	}

	return nil
}
