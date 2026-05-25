package engine

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"golang.org/x/sync/errgroup"
)

// LSPProvider defines the requirement for retrieving an LSP client.
type LSPProvider interface {
	GetClient(ctx context.Context, filePath string) (lsp.LSPClient, error)
}

// EnricherStore defines the data requirements for the Enricher.
type EnricherStore interface {
	store.SymbolRegistry
	store.StructuralGraph
	store.TransactionManager
}

// Enricher coordinates semantic enrichment of the call graph using LSP.
type Enricher struct {
	store EnricherStore
	lsp   LSPProvider
}

func NewEnricher(s EnricherStore, lp LSPProvider) *Enricher {
	return &Enricher{
		store: s,
		lsp:   lp,
	}
}

// Enrich methods with implementation data from LSP.
func (e *Enricher) Enrich(ctx context.Context) error {
	// 1. Get all methods (concrete implementations)
	methods, err := e.store.GetSymbolsByType(ctx, "method")
	if err != nil {
		return fmt.Errorf("failed to fetch methods: %w", err)
	}

	if len(methods) == 0 {
		return nil
	}

	type dynamicLink struct {
		interfaceMethod string
		structMethod    string
		structPath      string
		ifacePath       string
		ifaceLine       int
	}

	linksChan := make(chan dynamicLink, 100)
	g, groupCtx := errgroup.WithContext(ctx)

	// Worker Pool for LSP queries
	methodChan := make(chan store.Symbol, len(methods))
	for _, m := range methods {
		methodChan <- m
	}
	close(methodChan)

	workerCount := 4 // Optimal for local LSP servers
	for i := 0; i < workerCount; i++ {
		g.Go(func() error {
			for m := range methodChan {
				select {
				case <-groupCtx.Done():
					return groupCtx.Err()
				default:
				}

				client, err := e.lsp.GetClient(groupCtx, m.Path)
				if err != nil {
					continue
				}

				// Ensure character offset is not negative
				char := m.StartCol - 1
				if char < 0 {
					char = 0
				}

				params := lsp.ImplementationParams{
					TextDocumentPositionParams: lsp.TextDocumentPositionParams{
						TextDocument: lsp.TextDocumentIdentifier{
							URI: "file://" + m.Path,
						},
						Position: lsp.Position{
							Line:      m.StartLine - 1,
							Character: char,
						},
					},
				}

				locs, err := client.Implementation(groupCtx, params)
				if err != nil {
					continue
				}

				for _, loc := range locs {
					u, err := url.Parse(loc.URI)
					if err != nil || u.Scheme != "file" {
						continue
					}
					
					// Security: Project Jail validation
					targetPath := u.Path
					// Handle Windows drive letters in URI (file:///C:/...)
					if strings.Contains(targetPath, ":") && strings.HasPrefix(targetPath, "/") {
						targetPath = strings.TrimPrefix(targetPath, "/")
					}
					
					// Convert to relative if within repo to satisfy ValidatePath
					root, _ := utils.GetRepoRoot()
					if rel, err := filepath.Rel(root, targetPath); err == nil && !strings.HasPrefix(rel, "..") {
						targetPath = rel
					}

					validatedPath, err := utils.ValidatePath(targetPath)
					if err != nil {
						continue // Outside project jail
					}

					// Find the interface method at that location
					ifaceSyms, err := e.store.GetSymbolsByRange(groupCtx, validatedPath, loc.Range.Start.Line+1, loc.Range.End.Line+1)
					if err != nil || len(ifaceSyms) == 0 {
						continue
					}

					for _, ifaceSym := range ifaceSyms {
						if ifaceSym.Type != "method_spec" {
							continue
						}
						
						select {
						case linksChan <- dynamicLink{
							interfaceMethod: ifaceSym.Name,
							structMethod:    m.Name,
							structPath:      m.Path,
							ifacePath:       validatedPath,
							ifaceLine:       ifaceSym.StartLine,
						}:
						case <-groupCtx.Done():
							return groupCtx.Err()
						}
					}
				}
			}
			return nil
		})
	}

	// Collector goroutine
	go func() {
		_ = g.Wait()
		close(linksChan)
	}()

	// 2. Persist in a single transaction
	return e.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Store) error {
		for link := range linksChan {
			err := tx.SaveCall(txCtx, store.Call{
				CallerName: link.interfaceMethod,
				CalleeName: link.structMethod,
				CalleePath: link.structPath,
				Path:       link.ifacePath,
				Line:       link.ifaceLine,
				LinkType:   "dynamic",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}
