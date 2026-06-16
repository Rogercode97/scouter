package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Executes graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		var calls []store.Call
		filter := ""
		if len(args) > 0 {
			filter = args[0]
			results, err := db.GetCallees(cmd.Context(), filter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error fetching callees: %v\n", err)
				os.Exit(1)
				return nil
			}
			calls = results

			// Also get callers to show the immediate context
			callers, _ := db.GetCallers(cmd.Context(), filter, 50, 0)
			calls = append(calls, callers...)
		} else {
			// Get all calls (limited to prevent huge graphs)
			seq := db.GetAllCalls(cmd.Context())
			count := 0
			for call, err := range seq {
				if err != nil {
					break
				}
				calls = append(calls, call)
				count++
				if count > 500 { // Safety limit
					break
				}
			}
		}

		title := "Scouter Call Graph"
		if filter != "" {
			title = fmt.Sprintf("Graph for %s", filter)
		}

		mermaid := display.ExportMermaid(calls, title)
		fmt.Fprintln(os.Stdout, mermaid)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
}
