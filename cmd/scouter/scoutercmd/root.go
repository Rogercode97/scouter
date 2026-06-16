package scoutercmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Rogercode97/scouter/internal/cli"
	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/telemetry"
	"github.com/spf13/cobra"
	"log/slog"
)

var (
	cfg     *config.Config
	tracker *telemetry.Tracker

	// Global flags
	verbose      int
	ultraCompact bool
	enrich       bool
	deep         bool
	skipEnv      bool
	envFlag      string
	versionFlag  bool
)

var rootCmd = &cobra.Command{
	Use:   "scouter",
	Short: "scouter v" + cli.Version + " — CLI Token Killer & Oracle Engine",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cmd.Context())
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		tracker = telemetry.NewLazyTracker(cfg.Tracking.DBPath)
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if tracker != nil {
			tracker.Close()
		}
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "Enable detailed logging")
	rootCmd.PersistentFlags().BoolVarP(&ultraCompact, "ultra-compact", "u", false, "Maximize context efficiency in output")
	rootCmd.PersistentFlags().BoolVar(&enrich, "enrich", false, "Enable deep AST enrichment for proxied commands")
	rootCmd.PersistentFlags().BoolVar(&deep, "deep", false, "Enable deep AST enrichment for index")
	rootCmd.PersistentFlags().BoolVar(&skipEnv, "skip-env", false, "Skip environment variable checks")
	rootCmd.PersistentFlags().StringVar(&envFlag, "env", "production", "Set environment")
	rootCmd.PersistentFlags().BoolVar(&versionFlag, "version", false, "Print version")
}

func Execute(ctx context.Context, args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		rootCmd.SetArgs(args)
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		if err := rootCmd.ExecuteContext(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	_, _, err := rootCmd.Find(args)
	if err != nil && strings.HasPrefix(err.Error(), "unknown command") {
		return runProxy(ctx, args)
	}

	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runProxy(ctx context.Context, args []string) int {
	parsedFlags, remaining := cli.ParseFlags(args)
	if len(remaining) == 0 {
		if parsedFlags.Version {
			fmt.Fprintf(os.Stdout, "scouter v%s\n", cli.Version)
			return 0
		}
		rootCmd.Help()
		return 0
	}
	cmd := remaining[0]
	cmdArgs := remaining[1:]

	c, err := config.Load(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 1
	}

	t := telemetry.NewLazyTracker(c.Tracking.DBPath)
	defer t.Close()

	filters, err := filter.LoadAll(c.Filters.Dir)
	if err != nil {
		slog.Warn("could not load filters", "path", c.Filters.Dir, "error", err)
	}
	reg := filter.NewRegistry(filters)

	p := &engine.Pipeline{
		Registry:     reg,
		Tracker:      t,
		Verbose:      parsedFlags.Verbose,
		UltraCompact: parsedFlags.UltraCompact,
		Enrich:       parsedFlags.Enrich,
	}
	return p.Run(ctx, cmd, cmdArgs)
}

func openDB(ctx context.Context) (store.Store, func(), int) {
	db, err := store.NewStore(ctx, cfg.Tracking.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		return nil, nil, 1
	}
	return db, func() { db.Close() }, 0
}
