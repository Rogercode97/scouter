package engine

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

type Linker struct {
	logger *slog.Logger
}

func NewLinker(logger *slog.Logger) *Linker {
	return &Linker{logger: logger}
}

// LinkInterfaces performs semantic linking between interfaces and their implementations
// using the LSP 'textDocument/implementation' request.
func (l *Linker) LinkInterfaces(ctx context.Context, repo store.Store, lspMgr *lsp.Manager) error {
	interfaces, err := repo.GetInterfaces(ctx)
	if err != nil {
		l.logger.Error("failed to get interfaces", "error", err)
		return err
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

		// [Strike 6] Sincronización Determinista (LSP Black Box Sync)
		// gopls blocks Hover/Definition until the file is fully type-checked.
		// We send a dummy request to line 0 to wait for readiness instead of sleeping.
		syncParams := lsp.HoverParams{
			TextDocumentPositionParams: lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: "file://" + iface.Path,
				},
				Position: lsp.Position{Line: 0, Character: 0},
			},
		}
		_, _ = client.Hover(ctx, syncParams)

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
			l.logger.Error("LSP error", "interface", iface.Name, "error", err)
			continue
		}

		l.logger.Debug("Found implementations", "interface", iface.Name, "count", len(locations))

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
				l.logger.Error("SaveCall error", "interface", iface.Name, "error", err)
				continue
			}
		}
	}

	return nil
}
