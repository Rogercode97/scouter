package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Executes rollback",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		ledger := engine.NewLedger()
		ledger.SetLedgerPath(".scouter/staging/ledger.json")
		ee := engine.NewEvolutionEngine(db, ledger, nil)

		res, err := ee.RollbackLedger(cmd.Context())
		if err != nil {
			fmt.Fprintf(os.Stderr, "rollback error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintln(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
