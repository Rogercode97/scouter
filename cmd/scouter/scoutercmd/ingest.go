package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/telemetry/ingest"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Executes ingest",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if err := ingest.Ingest(cmd.Context(), os.Stdin, envFlag, db); err != nil {
			fmt.Fprintf(os.Stderr, "ingest error: %v\n", err)
			os.Exit(1)
			return nil
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}
