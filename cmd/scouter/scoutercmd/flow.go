package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/display"
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Executes flow",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "usage: scouter flow <symbol>\n")
			os.Exit(1)
			return nil
		}
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()
		flows, err := db.GetFlows(cmd.Context(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching flows: %v\n", err)
			os.Exit(1)
			return nil
		}
		display.PrintFlows(os.Stdout, args[0], flows)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(flowCmd)
}
