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
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/tee"
	"github.com/Rogercode97/scouter/internal/telemetry"
	"github.com/Rogercode97/scouter/internal/utils"
)

// version is set at build time via -ldflags "-X ...". Do not reassign.
var version = "2.5.1"

// Run is the main entry point. Returns exit code.
func Run(ctx context.Context, args []string) int {
	if len(args) < 2 {
		printUsage()
		return 0
	}

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

	if reason := unproxyableReason(command); reason != "" {
		fmt.Fprintf(os.Stderr, "scouter: %s cannot be proxied (%s)\n", command, reason)
		return 1
	}

	switch command {
	case "init":
		if err := initcmd.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "predict":
		return runPredict(ctx)

	case "strike":
		return runStrike(ctx, flags)

	case "critical":
		return runCritical(ctx)

	case "sync":
		return runSync(ctx, cmdArgs)

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
		if len(cmdArgs) == 0 {
			display.PrintError("proxy requires a command argument")
			return 1
		}
		p := &engine.Pipeline{}
		return p.Passthrough(ctx, cmdArgs[0], cmdArgs[1:])
	}

	return runPipeline(ctx, command, cmdArgs, flags)
}

func getOraclePredictions(ctx context.Context) (map[string]bool, error) {
	cfg, _ := config.Load(ctx)
	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	changes, err := utils.GetLocalChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("git error: %v", err)
	}

	if len(changes) == 0 {
		return nil, nil
	}

	suggestedTests := make(map[string]bool)
	fmt.Printf("\n--- 🔮 Oracle Prediction (Staged Changes) ---\n")
	
	for _, change := range changes {
		absPath, _ := os.Getwd()
		absPath = fmt.Sprintf("%s/%s", absPath, change.Path)
		
		symbols, _ := db.GetSymbolsByRange(ctx, absPath, change.StartLine, change.EndLine)
		for _, sym := range symbols {
			fmt.Printf("  • Symbol Affected: %s [%s]\n", sym.Name, change.Path)
			impact, _ := db.GetImpact(ctx, sym.Name, sym.Path, 5)
			for _, imp := range impact {
				if strings.HasPrefix(imp.Symbol, "Test") || strings.HasSuffix(imp.File, "_test.go") {
					suggestedTests[imp.Symbol] = true
				}
			}
		}
	}
	return suggestedTests, nil
}

func runPredict(ctx context.Context) int {
	suggestedTests, err := getOraclePredictions(ctx)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	if len(suggestedTests) == 0 {
		fmt.Println("No local changes detected (Clean Staging).")
		return 0
	}

	fmt.Println("\n--- 🎯 Suggested Tests to Run ---")
	for t := range suggestedTests {
		fmt.Printf("  go test -v -run %s\n", t)
	}
	return 0
}

func runStrike(ctx context.Context, flags Flags) int {
	suggestedTests, err := getOraclePredictions(ctx)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	if len(suggestedTests) == 0 {
		fmt.Println("No local changes detected (Clean Staging).")
		return 0
	}

	fmt.Println("\n--- ⚡ Oracle Strike (Executing Tests) ---")
	var testNames []string
	for t := range suggestedTests {
		testNames = append(testNames, t)
	}
	
	runRegex := "^(" + strings.Join(testNames, "|") + ")$"
	strikeArgs := []string{"test", "-v", "-run", runRegex, "./..."}
	
	// DIVINE REDEMPTION: Use the pipeline for Strike so it gets passive health ingestion
	return runPipeline(ctx, "go", strikeArgs, flags)
}

func runSync(ctx context.Context, args []string) int {
	cfg, _ := config.Load(ctx)
	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}
	defer db.Close()

	syncDir := ".scouter/sync"
	mode := "--push"
	if len(args) > 0 {
		mode = args[0]
	}

	switch mode {
	case "--push":
		fmt.Printf("Exporting Sovereign Delta to %s...\n", syncDir)
		if err := db.ExportDelta(ctx, syncDir); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		fmt.Println("✅ Index exported. Ready for 'git add .scouter/sync'.")
	case "--pull":
		fmt.Printf("Importing Sovereign Delta from %s...\n", syncDir)
		if err := db.ImportDelta(ctx, syncDir); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		fmt.Println("✅ Local index hydrated.")
	default:
		display.PrintError("Invalid sync mode. Use --push or --pull.")
		return 1
	}
	return 0
}

func runCritical(ctx context.Context) int {
	cfg, _ := config.Load(ctx)
	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}
	defer db.Close()

	critical, err := db.GetCriticalSymbols(ctx, 10)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	fmt.Printf("\n--- 📊 Risk Map: Top 10 Critical Symbols ---\n")
	fmt.Printf("%-25s | %-10s | %-10s\n", "SYMBOL", "CENTRALITY", "FRAGILITY")
	fmt.Println(strings.Repeat("-", 50))
	for _, s := range critical {
		fmt.Printf("%-25s | %-10d | %-10d\n", s.Name, s.Centrality, s.Fragility)
	}
	return 0
}

func runPipeline(ctx context.Context, command string, args []string, flags Flags) int {
	cfg, err := config.Load(ctx)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dir)
	if err != nil {
		display.PrintError(fmt.Sprintf("load filters: %v", err))
		return 1
	}

	registry := filter.NewRegistry(filters)

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
	usage := `scouter v%s — CLI Token Killer & Oracle Engine

Usage: scouter [flags] <command> [args...]

Intelligence Commands:
  predict      🔮 Identify affected tests from staged changes
  strike       ⚡ Execute affected tests automatically
  critical     📊 Show the project risk map (top critical symbols)
  sync         📦 Distributed index synchronization (--push, --pull)

System Commands:
  init         Install Scouter hooks
  gain         Show token savings report
  config       Show current configuration
  proxy        Passthrough without filtering

Pipeline Commands:
  <command>    Run command through scouter filter pipeline

Flags:
  -v, -vv      Verbose output
  -u            Ultra-compact mode
  --version     Show version
  --help        Show this help
`
	fmt.Printf(usage, version)
}

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
