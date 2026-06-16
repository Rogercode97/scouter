package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/spf13/cobra"
)

var propagateCmd = &cobra.Command{
	Use:   "propagate",
	Short: "Executes propagate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: scouter propagate <symbol> <transformation>\n")
			os.Exit(1)
			return nil
		}
		symbol := args[0]
		transformation := args[1]

		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()

		impactEngine := engine.NewImpactEngine(db, lspMgr, nil)
		rippleEngine := engine.NewRippleEngine(db, nil, impactEngine)
		ledger := engine.NewLedger()
		ledger.SetLedgerPath(".scouter/staging/ledger.json")

		te := engine.NewTruthEngine(db, engine.WithRipple(rippleEngine), engine.WithLedger(ledger))

		res, err := te.Propagate(cmd.Context(), symbol, transformation, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "propagate error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintln(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(propagateCmd)
}
