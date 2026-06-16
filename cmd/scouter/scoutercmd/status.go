package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Executes status",
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

		res := te.GetLedgerSummary(cmd.Context())
		fmt.Fprintln(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
