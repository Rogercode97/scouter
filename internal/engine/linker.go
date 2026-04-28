package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

// LinkInterfaces performs semantic interface resolution using LSP.
// It replaces the legacy nominal matching in store.go to provide Wave 11 fidelity.
func LinkInterfaces(ctx context.Context, repo store.Repository, lspMgr *lsp.Manager) error {
	// 1. Fetch all interfaces from the store
	interfaces, err := repo.GetInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch interfaces: %w", err)
	}

	if len(interfaces) == 0 {
		return nil
	}

	// 2. Resolve implementations via LSP
	return repo.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
		for _, iface := range interfaces {
			select {
			case <-txCtx.Done():
				return txCtx.Err()
			default:
			}

			client, err := lspMgr.GetClient(txCtx, iface.Path)
			if err != nil {
				// Skip if LSP is not available for this file
				continue
			}

			// Mandate: LSP Timeout (2s) to prevent blocking the indexer
			queryCtx, cancel := context.WithTimeout(txCtx, 2*time.Second)
			locs, err := client.Implementation(queryCtx, lsp.ImplementationParams{
				TextDocumentPositionParams: lsp.TextDocumentPositionParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + iface.Path},
					Position: lsp.Position{
						Line:      iface.StartLine - 1,
						Character: 0, // Heuristic: start of the line for the interface keyword
					},
				},
			})
			cancel()

			if err != nil {
				continue
			}

			for _, loc := range locs {
				implPath := strings.TrimPrefix(loc.URI, "file://")
				// Normalize path for the database
				absImplPath, err := filepath.Abs(implPath)
				if err != nil {
					absImplPath = implPath
				}

				// Fetch the symbol at the implementation location to get its name
				implSymbols, err := tx.GetSymbolsByRange(txCtx, absImplPath, loc.Range.Start.Line+1, loc.Range.End.Line+1)
				if err != nil || len(implSymbols) == 0 {
					continue
				}

				// Create the "implements" link
				// We use the first symbol found at that range (usually the struct/type)
				targetSym := implSymbols[0]
				err = tx.SaveCall(txCtx, store.Call{
					CallerName: targetSym.Name,
					CalleeName: iface.Name,
					Path:       absImplPath,
					Line:       targetSym.StartLine,
					CalleePath: iface.Path,
					LinkType:   "implements",
				})
				if err != nil {
					return fmt.Errorf("failed to save implements link for %s: %w", targetSym.Name, err)
				}
			}
		}
		return nil
	})
}
