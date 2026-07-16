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
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, impactEngine, lspMgr, searchEngine)
		healer := engine.NewHealerEngine(db, lspMgr, analyzer, impactEngine, searchEngine, nil, diagnostic)

		report, err := diagnostic.Diagnose(cmd.Context(), string(logBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "fix error (diagnose): %v\n", err)
			os.Exit(1)
			return nil
		}

		fixRes, err := healer.Fix(cmd.Context(), report.ErrorLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fix error: %v\n", err)
			os.Exit(1)
			return nil
		}

		res := fmt.Sprintf("Status: %s\nFile: %s\nFixed Code:\n%s\nTest Output:\n%s", fixRes.Status, fixRes.Metadata["failingFile"], fixRes.FixedCode, fixRes.TestOutput)

		fmt.Fprintf(os.Stdout, "%s\n", res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(fixCmd)
}
