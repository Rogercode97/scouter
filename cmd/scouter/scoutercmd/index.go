package scoutercmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Executes index",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "usage: scouter index <path> [--deep]\n")
			os.Exit(1)
			return nil
		}

		path := args[0]
		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()

		analyzer := engine.NewAnalysisEngine(db)
		te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer), engine.WithLSP(lspMgr))

		start := time.Now()
		if err := te.Index(cmd.Context(), path); err != nil {
			fmt.Fprintf(os.Stderr, "index error: %v\n", err)
			os.Exit(1)
			return nil
		}

		// Mode Deep: High-Precision SSA Analysis (Go only)
		if deep {
			fmt.Fprintf(os.Stdout, "🛡️  Running Mode Deep (SSA Analysis)...\n")

			// Ensure we have a directory for SSA loading
			absPath, _ := filepath.Abs(path)
			info, err := os.Stat(absPath)
			if err == nil && !info.IsDir() {
				absPath = filepath.Dir(absPath)
			}

			ssaCalls, err := engine.SSACallGraph(cmd.Context(), absPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "SSA analysis error: %v\n", err)
				// Don't fail the whole index if SSA fails (it's a best-effort deep dive)
			} else {
				for _, c := range ssaCalls {
					_ = db.SaveCall(cmd.Context(), store.Call{
						CallerName: c.CallerName,
						CalleeName: c.CalleeName,
						LinkType:   c.LinkType,
						Path:       c.Path,
						Line:       c.Line,
					})
				}
				fmt.Fprintf(os.Stdout, "✨ Deep Analysis complete (%d high-precision calls found)\n", len(ssaCalls))
			}
		}

		duration := time.Since(start).Milliseconds()

		fc, sc, err := db.GetStats(cmd.Context())
		if err == nil && tracker != nil {
			savedTokens := fc*1500 + sc*100
			_ = tracker.Track(cmd.Context(), "scouter index "+path, "scouter index "+path, savedTokens, 5, duration)
		}

		fmt.Printf("✅ Indexed %s\n", path)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
