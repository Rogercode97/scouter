package scoutercmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context <file>",
	Short: "Single-call architectural context packet for AI agents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		targetFile := args[0]
		contextEngine := engine.NewContextEngine(db)

		res, err := contextEngine.BuildFileContext(cmd.Context(), targetFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "context error: %v\n", err)
			os.Exit(1)
			return nil
		}

		if ultraCompact {
			data, err := json.Marshal(res)
			if err != nil {
				fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
				os.Exit(1)
				return nil
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		fmt.Fprintf(os.Stdout, "File: %s (LOC: %d, Symbols: %d)\n", res.File, res.LOC, res.Symbols)
		fmt.Fprintf(os.Stdout, "Verdict: %s (Risk: %s, Churn: %.2f)\n", res.Verdict, res.EditCost.Risk, res.ChurnScore)
		fmt.Fprintf(os.Stdout, "Edit Cost: ~%d tokens (Files at risk: %d, Cascade: %d)\n", res.EditCost.Tokens, res.EditCost.Files, res.EditCost.Cascade)

		if len(res.DirectDependents) > 0 {
			fmt.Fprintf(os.Stdout, "Direct Dependents:\n")
			for _, dep := range res.DirectDependents {
				fmt.Fprintf(os.Stdout, "  - %s\n", dep)
			}
		}

		if len(res.TestHints) > 0 {
			fmt.Fprintf(os.Stdout, "Test Hints:\n")
			for _, th := range res.TestHints {
				fmt.Fprintf(os.Stdout, "  - %s\n", th)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
