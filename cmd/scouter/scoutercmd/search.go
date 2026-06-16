package scoutercmd

import (
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Executes search",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "usage: scouter search <query>\n")
			os.Exit(1)
			return nil
		}

		query := args[0]
		search := engine.NewSearchEngine(db, nil, nil)

		results, err := search.HybridSearch(cmd.Context(), query, 10, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "search error: %v\n", err)
			os.Exit(1)
			return nil
		}

		if len(results.Symbols) == 0 {
			fmt.Fprintf(os.Stdout, "No results found for %q\n", query)
			return nil
		}

		fmt.Fprintf(os.Stdout, "🔍 Search results for %q:\n", query)
		for _, sym := range results.Symbols {
			fmt.Fprintf(os.Stdout, "- [%s] %s (%s:%d)\n", sym.Type, sym.Name, sym.Path, sym.StartLine)
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
