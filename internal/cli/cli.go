package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/initcmd"
	"github.com/Rogercode97/scouter/internal/tee"
	"github.com/Rogercode97/scouter/internal/telemetry"
)

// version is set at build time via -ldflags "-X ...". Do not reassign.
var version = "dev"

// Run is the main entry point. Returns exit code.
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 0
	}

	ctx := context.Background()
	flags, remaining := ParseFlags(args[1:])

	if flags.Version {
		fmt.Printf("scouter v%s\n", version)
		return 0
	}
	if flags.Help || len(remaining) == 0 {
		printUsage()
		return 0
	}

	command := remaining[0]
	cmdArgs := remaining[1:]

	// Commands that cannot be proxied: they must run in the parent shell
	// to have any effect. Running them in a subprocess is a silent no-op.
	if unproxyableReason(command) != "" {
		fmt.Fprintf(os.Stderr, "scouter: %s cannot be proxied (%s)\n", command, unproxyableReason(command))
		return 1
	}

	// Built-in commands
	switch command {
	case "init":
		if err := initcmd.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "gain":
		if !telemetry.DriverAvailable {
			display.PrintError("gain requires full build (this binary was built with -tags lite)")
			return 1
		}
		cfg, cfgErr := config.Load(ctx)
		if cfgErr != nil {
			cfg = config.DefaultConfig()
		}
		dbPath := telemetry.DBPath(cfg.Tracking.DBPath)
		tracker, err := telemetry.NewTracker(ctx, dbPath)
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		defer func() { _ = tracker.Close() }()
		if err := display.RunGain(tracker, cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "config":
		cfg, err := config.Load(ctx)
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		fmt.Printf("telemetry.db_path: %s\n", cfg.Tracking.DBPath)
		fmt.Printf("filters.dir: %s\n", cfg.Filters.Dir)
		fmt.Printf("tee.mode: %s\n", cfg.Tee.Mode)
		fmt.Printf("tee.max_files: %d\n", cfg.Tee.MaxFiles)
		fmt.Printf("display.color: %v\n", cfg.Display.Color)
		fmt.Printf("display.emoji: %v\n", cfg.Display.Emoji)
		return 0

	case "proxy":
		// Direct passthrough without filtering
		if len(cmdArgs) == 0 {
			display.PrintError("proxy requires a command argument")
			return 1
		}
		p := &engine.Pipeline{}
		return p.Passthrough(ctx, cmdArgs[0], cmdArgs[1:])
	}

	// Filter pipeline
	return runPipeline(ctx, command, cmdArgs, flags)
}

func runPipeline(ctx context.Context, command string, args []string, flags Flags) int {
	cfg, err := config.Load(ctx)
	if err != nil {
		if flags.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: config error: %v, using defaults\n", err)
		}
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dir)
	if err != nil {
		display.PrintError(fmt.Sprintf("load filters: %v", err))
		return 1
	}

	registry := filter.NewRegistry(filters)

	// Lazy tracker: DB opens on first use (concurrently with command execution)
	var tracker *telemetry.Tracker
	if telemetry.DriverAvailable {
		dbPath := telemetry.DBPath(cfg.Tracking.DBPath)
		tracker = telemetry.NewLazyTracker(dbPath)
		defer func() { _ = tracker.Close() }()
	}

	teeCfg := tee.DefaultConfig()
	teeCfg.Enabled = cfg.Tee.Enabled
	teeCfg.Mode = cfg.Tee.Mode
	teeCfg.MaxFiles = cfg.Tee.MaxFiles
	teeCfg.MaxFileSize = cfg.Tee.MaxFileSize

	pipeline := &engine.Pipeline{
		Registry:     registry,
		Tracker:      tracker,
		TeeConfig:    teeCfg,
		Verbose:      flags.Verbose,
		UltraCompact: flags.UltraCompact,
	}

	return pipeline.Run(ctx, command, args)
}

func printUsage() {
	usage := `scouter v%s — CLI Token Killer

Usage: scouter [flags] <command> [args...]

Commands:
  <command>    Run command through scouter filter pipeline
  init         Install Claude Code hook
  gain         Show token savings report
  config       Show current configuration
  proxy        Passthrough without filtering

Flags:
  -v, -vv      Verbose output (stackable)
  -u            Ultra-compact mode
  --skip-env    Skip environment loading
  --version     Show version
  --help        Show this help

Examples:
  scouter git log -10
  scouter go test ./...
  scouter gain --daily
  scouter gain --weekly
  scouter gain --monthly
  scouter gain --top 10
  scouter gain --history 20
  scouter init
`
	fmt.Printf(usage, version)
}

// unproxyableReason returns a human-readable reason if the command cannot be
// proxied through an external process, or "" if it can.
func unproxyableReason(command string) string {
	switch command {
	case "cd":
		return "it must run in the parent shell to change directory"
	case "source", ".":
		return "it must run in the parent shell to modify the environment"
	}
	return ""
}

// Version returns the current version string.
func Version() string {
	return version
}

// BuildCommandString joins command and args for display.
func BuildCommandString(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return command + " " + strings.Join(args, " ")
}
