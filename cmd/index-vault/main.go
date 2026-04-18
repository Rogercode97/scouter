package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
)

func main() {
	ctx := context.Background()
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".scouter", "scouter.db")

	s, err := store.New(ctx, dbPath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()
	vaultPath, _ := os.Getwd()
	fmt.Printf("--- Indexing Workspace: %s ---\n", vaultPath)

	err = filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		ext := filepath.Ext(path)
		if !info.IsDir() && (ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" || ext == ".py") {
			fmt.Printf("Indexing: %s\n", path)

			// 1. Calculate Hash
			h, hashErr := utils.CalculateHash(path)
			if hashErr != nil {
				fmt.Printf("  [Error] Hash failed: %v\n", hashErr)
				return nil
			}

			// 2. Parse (Definitions + Calls)
			syms, calls, parseErr := engine.ParseFile(ctx, path)
			if parseErr != nil {
				fmt.Printf("  [Error] Parse failed: %v\n", parseErr)
				return nil
			}

			// 3. Save Index, Symbols and Calls Atomically
			err = s.WithTransaction(ctx, func(tx store.Repository) error {
				if err := tx.SaveFileIndex(ctx, &store.FileIndex{
					Path:  path,
					Mtime: info.ModTime().UnixNano(),
					Hash:  h,
				}); err != nil {
					return err
				}

				if err := tx.ClearSymbols(ctx, path); err != nil {
					return err
				}
				if err := tx.ClearCalls(ctx, path); err != nil {
					return err
				}

				for _, sym := range syms {
					if err := tx.SaveSymbol(ctx, &store.Symbol{
						Name:      sym.Name,
						Type:      sym.Type,
						Path:      path,
						StartByte: sym.Range.Start,
						EndByte:   sym.Range.End,
						StartLine: sym.StartLine,
						EndLine:   sym.EndLine,
					}); err != nil {
						return err
					}
				}

				for _, call := range calls {
					if err := tx.SaveCall(ctx, store.Call{
						CallerName: call.CallerName,
						CalleeName: call.CalleeName,
						Path:       call.Path,
						Line:       call.Line,
					}); err != nil {
						return err
					}
				}
				return nil
			})

			if err != nil {
				fmt.Printf("  [Error] Transaction failed: %v\n", err)
				return nil
			}

			fmt.Printf("  [Success] Indexed %d symbols and %d calls.\n", len(syms), len(calls))
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Walk failed: %v", err)
	}

	fmt.Println("--- Vault Indexing Complete. ---")
}
