package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Executes fix",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "usage: scouter fix <error_log_file>\n")
			os.Exit(1)
			return nil
		}

		logBytes, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read error log: %v\n", err)
			os.Exit(1)
			return nil
		}

		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()

		analyzer := engine.NewAnalysisEngine(db)
		impactEngine := engine.NewImpactEngine(db, lspMgr, nil)
		searchEngine := engine.NewSearchEngine(db, nil, nil)
		healer := engine.NewHealerEngine(db, lspMgr, analyzer, impactEngine, searchEngine, nil)
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, impactEngine, healer, lspMgr, searchEngine)

		te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer), engine.WithLSP(lspMgr), engine.WithImpact(impactEngine), engine.WithDiagnostic(diagnostic), engine.WithHealer(healer), engine.WithSearch(searchEngine))

		res, err := te.Fix(cmd.Context(), string(logBytes), nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fix error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintf(os.Stdout, "%s\n", res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(fixCmd)
}
