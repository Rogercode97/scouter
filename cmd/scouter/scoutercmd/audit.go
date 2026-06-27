package scoutercmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Executes audit",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		targetPath := "."
		if len(args) > 0 {
			targetPath = args[0]
		}

		rulesDir := filepath.Join(".", "internal", "filters", "rules") // Default or from config
		ruleEngine := engine.NewASTRuleEngine(rulesDir)

		matches, err := ruleEngine.Audit(cmd.Context(), targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit error: %v\n", err)
			os.Exit(1)
			return nil
		}

		if len(matches) == 0 {
			fmt.Fprintf(os.Stdout, "✅ No architectural violations found.\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "❌ Architectural Violations Found:\n")
		for _, m := range matches {
			fmt.Fprintf(os.Stdout, "  - [%s] %s: %s (Line %d)\n", m.Severity, m.RuleID, m.File, m.Range.Start.Line)
		}
		os.Exit(1)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
