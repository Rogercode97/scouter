package scoutercmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/spf13/cobra"
)

var predictCmd = &cobra.Command{
	Use:   "predict",
	Short: "Executes predict",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		diff := ""
		if len(args) > 0 {
			diff = args[0]
		} else {
			out, err := exec.CommandContext(cmd.Context(), "git", "diff", "HEAD", "--unified=0").Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting git diff: %v\n", err)
				os.Exit(1)
				return nil
			}
			diff = string(out)
		}

		ie := engine.NewImpactEngine(db, nil, nil)
		start := time.Now()
		results, err := ie.PredictTests(cmd.Context(), diff)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prediction error: %v\n", err)
			os.Exit(1)
			return nil
		}
		duration := time.Since(start).Milliseconds()

		outputStr := ""
		if len(results) == 0 {
			outputStr = "No affected tests identified.\n"
		} else {
			outputStr = "🎯 Affected Tests:\n"
			for _, r := range results {
				outputStr += fmt.Sprintf("- %s (%s)\n", r.Name, r.File)
			}
		}

		if tracker != nil {
			savedTokens := 4000 + len(results)*500
			outputTokens := utils.EstimateTokens(outputStr)
			_ = tracker.Track(cmd.Context(), "scouter predict", "scouter predict", savedTokens, outputTokens, duration)
		}

		fmt.Fprint(os.Stdout, outputStr)
		return nil

	},
}

func init() {
	rootCmd.AddCommand(predictCmd)
}
