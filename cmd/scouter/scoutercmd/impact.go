package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Executes impact",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: scouter impact <symbol> <path>\n")
			os.Exit(1)
			return nil
		}

		symbol := args[0]
		path := args[1]

		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()

		analyzer := engine.NewAnalysisEngine(db)
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, nil, nil, lspMgr, nil)
		impactEngine := engine.NewImpactEngine(db, lspMgr, nil)
		te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer), engine.WithLSP(lspMgr), engine.WithImpact(impactEngine), engine.WithDiagnostic(diagnostic))

		res, err := te.AnalyzeImpact(cmd.Context(), symbol, path, verbose > 0, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "impact error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintf(os.Stdout, "Risk Level: %s\n", res.RiskLevel)
		fmt.Fprintf(os.Stdout, "Blast Radius: %d callers\n", len(res.Callers))
		if len(res.Callers) > 0 {
			fmt.Fprintf(os.Stdout, "Affected Callers:\n")
			for _, c := range res.Callers {
				fmt.Fprintf(os.Stdout, "  - %s (%s)\n", c.Symbol, c.File)
			}
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(impactCmd)
}
