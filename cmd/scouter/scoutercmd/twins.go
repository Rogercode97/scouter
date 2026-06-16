package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var twinsCmd = &cobra.Command{
	Use:   "twins",
	Short: "Executes twins",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: scouter twins <symbol> <path>\n")
			os.Exit(1)
			return nil
		}

		symbol := args[0]
		path := args[1]

		te := engine.NewTruthEngine(db)

		results, err := te.FindLogicalTwins(cmd.Context(), symbol, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "twins error: %v\n", err)
			os.Exit(1)
			return nil
		}

		if len(results) == 0 {
			fmt.Fprintf(os.Stdout, "No twins found for %s in %s\n", symbol, path)
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
	rootCmd.AddCommand(twinsCmd)
}
