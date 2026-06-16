package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var neighborhoodCmd = &cobra.Command{
	Use:   "neighborhood",
	Short: "Executes neighborhood",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "usage: scouter neighborhood <file>\n")
			os.Exit(1)
			return nil
		}

		filePath := args[0]
		analyzer := engine.NewAnalysisEngine(db)
		te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer))

		res, err := te.GetNeighborhood(cmd.Context(), filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "neighborhood error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprint(os.Stdout, res)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(neighborhoodCmd)
}
