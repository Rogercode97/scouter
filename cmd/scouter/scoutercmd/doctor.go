package scoutercmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/spf13/cobra"
)

// doctorCheck is one read-only health probe. ok describes the healthy state;
// a non-nil error carries the remedy.
type doctorCheck struct {
	name string
	run  func(ctx context.Context) (ok string, err error)
}

// runDoctorChecks executes every probe and renders one line each.
// It returns true when at least one probe failed.
func runDoctorChecks(ctx context.Context, out *os.File, checks []doctorCheck) bool {
	failed := false
	for _, c := range checks {
		ok, err := c.run(ctx)
		if err != nil {
			failed = true
			fmt.Fprintf(out, "FAIL %s — %v\n", c.name, err)
			continue
		}
		fmt.Fprintf(out, "ok   %s — %s\n", c.name, ok)
	}
	return failed
}

func defaultDoctorChecks(dbPath string) []doctorCheck {
	return []doctorCheck{
		{
			name: "ast-grep",
			run: func(ctx context.Context) (string, error) {
				bin, err := utils.AstGrepBinary()
				if err != nil {
					return "", fmt.Errorf("not on PATH: install sg or ast-grep (remedy: CI installs it; local: your package manager)")
				}
				return fmt.Sprintf("resolves as %q (needed by propagate/diff staging)", bin), nil
			},
		},
		{
			name: "database",
			run: func(ctx context.Context) (string, error) {
				db, err := store.NewStore(ctx, dbPath)
				if err != nil {
					return "", fmt.Errorf("cannot open %s: %v (remedy: scouter index .)", dbPath, err)
				}
				defer db.Close()
				return fmt.Sprintf("opens at %s (remedy if empty: scouter index .)", dbPath), nil
			},
		},
		{
			name: "ledger",
			run: func(ctx context.Context) (string, error) {
				ledger := engine.NewLedger()
				ledger.SetLedgerPath(".scouter/staging/ledger.json")
				if err := ledger.Load(); err != nil {
					if os.IsNotExist(err) {
						return "no staging file, nothing pending", nil
					}
					return "", fmt.Errorf("staging file unreadable: %v (remedy: scouter rollback clears it)", err)
				}
				if ledger.HasInterruptedCommit() {
					return "", fmt.Errorf("interrupted commit pending on %d files (remedy: inspect, then scouter rollback or re-commit)", len(ledger.InterruptedAppliedFiles()))
				}
				staged := ledger.StagedFiles()
				if len(staged) > 0 {
					return fmt.Sprintf("%d files staged, review with scouter diff", len(staged)), nil
				}
				return "no staging file, nothing pending", nil
			},
		},
	}
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report-only health checks: ast-grep, database, ledger",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runDoctorChecks(cmd.Context(), os.Stdout, defaultDoctorChecks(cfg.Tracking.DBPath)) {
			os.Exit(1)
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
