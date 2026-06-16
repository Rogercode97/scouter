package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Executes diff",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		ledger := engine.NewLedger()
		ledger.SetLedgerPath(".scouter/staging/ledger.json")
		te := engine.NewTruthEngine(db, engine.WithLedger(ledger))

		res, err := te.GetLedgerDiff(cmd.Context())
		if err != nil {
			fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintln(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
