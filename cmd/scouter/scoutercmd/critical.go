package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var criticalCmd = &cobra.Command{
	Use:   "critical",
	Short: "Executes critical",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		limit := 10
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &limit)
		}

		analyzer := engine.NewAnalysisEngine(db)
		results, err := analyzer.GetCriticalSymbols(cmd.Context(), limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "critical symbols error: %v\n", err)
			os.Exit(1)
			return nil
		}

		if len(results) == 0 {
			fmt.Fprintf(os.Stdout, "No critical symbols found.\n")
			return nil
		}

		zonStr, err := engine.EncodeZON(results)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zon encode error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprint(os.Stdout, zonStr)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(criticalCmd)
}
