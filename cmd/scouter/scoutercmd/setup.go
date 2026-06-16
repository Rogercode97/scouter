package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/initcmd"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Executes setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initcmd.Run(args); err != nil {
			fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
			os.Exit(1)
			return nil
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
