package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Executes commit",
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

		res, err := te.CommitLedger(cmd.Context())
		if err != nil {
			fmt.Fprintf(os.Stderr, "commit error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintln(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
