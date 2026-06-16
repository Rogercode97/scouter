package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/display"
	"github.com/spf13/cobra"
)

var gainCmd = &cobra.Command{
	Use:   "gain",
	Short: "Executes gain",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := display.RunGain(tracker, args); err != nil {
			fmt.Fprintf(os.Stderr, "error running gain: %v\n", err)
			os.Exit(1)
			return nil
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(gainCmd)
}
