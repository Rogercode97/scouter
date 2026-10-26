package scoutercmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/spf13/cobra"
)

// leftoverMentions lists staged files where a rewrite happened but the old
// symbol name survived — typically qualified call sites (`pkg.Old`) the
// `$SYMBOL` pattern does not match. Files staged byte-identical are ignored:
// nothing was attempted there, so nothing is half-done.
func leftoverMentions(ledger *engine.Ledger, oldName string) []string {
	var leftovers []string
	for path, p := range ledger.Staged {
		if p.NewContent == p.Original {
			continue
		}
		if strings.Contains(p.NewContent, oldName) {
			leftovers = append(leftovers, path)
		}
	}
	return leftovers
}

// parsePropagateTransformation maps the CLI transformation form to the
// ast-grep rewrite template the StructuralTransformer applies. The only form
// the CLI supports is 'rename:NewName', applied to every occurrence of the
// symbol ('$SYMBOL' pattern).
func parsePropagateTransformation(transformation string) (string, error) {
	name, ok := strings.CutPrefix(transformation, "rename:")
	if !ok || name == "" {
		return "", fmt.Errorf("unsupported transformation %q: usage: scouter propagate <symbol> 'rename:NewName'", transformation)
	}
	return name, nil
}

var propagateCmd = &cobra.Command{
	Use:   "propagate",
	Short: "Executes propagate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: scouter propagate <symbol> 'rename:NewName'\n")
			os.Exit(1)
			return nil
		}
		symbol := args[0]
		transformation, err := parsePropagateTransformation(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "propagate error: %v\n", err)
			os.Exit(1)
			return nil
		}

		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()

		impactEngine := engine.NewImpactEngine(db, lspMgr, nil)
		rippleEngine := engine.NewRippleEngine(db, &engine.StructuralTransformer{Pattern: "$SYMBOL"}, impactEngine)
		ledger := engine.NewLedger()
		ledger.SetLedgerPath(".scouter/staging/ledger.json")

		ee := engine.NewEvolutionEngine(db, ledger, rippleEngine)

		res, err := ee.Propagate(cmd.Context(), symbol, transformation, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "propagate error: %v\n", err)
			os.Exit(1)
			return nil
		}

		fmt.Fprintln(os.Stdout, res)

		oldLocal := symbol
		if i := strings.LastIndex(symbol, "."); i >= 0 {
			oldLocal = symbol[i+1:]
		}
		staged := engine.NewLedger()
		staged.SetLedgerPath(".scouter/staging/ledger.json")
		if err := staged.Load(); err == nil {
			if left := leftoverMentions(staged, oldLocal); len(left) > 0 {
				fmt.Fprintf(os.Stdout, "⚠️  %q still appears in %d staged files (qualified call sites the $SYMBOL pattern does not rewrite): %v\nReview them before scouter commit.\n", oldLocal, len(left), left)
			}
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(propagateCmd)
}
