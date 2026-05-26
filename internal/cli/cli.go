package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/initcmd"
	"github.com/Rogercode97/scouter/internal/mcp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/telemetry"
	"github.com/Rogercode97/scouter/internal/telemetry/ingest"
	"github.com/Rogercode97/scouter/internal/utils"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

type App struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (app *App) printUsage() {
	usage := `scouter v%s — CLI Token Killer & Oracle Engine
... (usage text) ...
`
	fmt.Fprintf(app.Stdout, usage, version)
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	app := &App{Stdout: stdout, Stderr: stderr}

	flags, remaining := ParseFlags(args[1:])
	if len(remaining) == 0 {
		if flags.Version {
			fmt.Fprintf(stdout, "scouter v%s\n", version)
			return 0
		}
		app.printUsage()
		return 0
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	// Foundation: Load mandatory configuration
	cfg, err := config.Load(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error loading config: %v\n", err)
		return 1
	}

	// Telemetry: Lazy tracker to avoid SQLite locks on fast paths
	tracker := telemetry.NewLazyTracker(cfg.Tracking.DBPath)
	defer tracker.Close()

	switch cmd {
	case "mcp":
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		server := mcp.NewServer(db, logger)
		defer server.Close()
		
		transport := &sdk.StdioTransport{}
		if err := server.Start(ctx, transport); err != nil {
			fmt.Fprintf(stderr, "MCP Server stopped: %v\n", err)
		}
		return 0

	case "index":
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		if len(cmdArgs) == 0 {
			fmt.Fprintf(stderr, "usage: scouter index <path>\n")
			return 1
		}

		// Use TruthEngine for unified indexing (Deepening)
		path := cmdArgs[0]
		
		lspMgr := lsp.NewManager()
		defer lspMgr.Close()
		
		analyzer := engine.NewAnalysisEngine(db)
		te := engine.NewTruthEngine(db, nil, analyzer, lspMgr, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		
		start := time.Now()
		if err := te.Index(ctx, path); err != nil {
			fmt.Fprintf(stderr, "index error: %v\n", err)
			return 1
		}
		duration := time.Since(start).Milliseconds()

		fc, sc, err := db.GetStats(ctx)
		if err == nil && tracker != nil {
			// Each file saves reading context (~1500 tokens), each symbol saves AST search context (~100 tokens)
			savedTokens := fc*1500 + sc*100
			_ = tracker.Track(ctx, "scouter index "+path, "scouter index "+path, savedTokens, 5, duration)
		}

		fmt.Printf("✅ Indexed %s\n", path)
		return 0

	case "ingest":
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		if err := ingest.Ingest(ctx, os.Stdin, flags.Env, db); err != nil {
			fmt.Fprintf(stderr, "ingest error: %v\n", err)
			return 1
		}
		return 0

	case "gain":
		if err := display.RunGain(tracker, cmdArgs); err != nil {
			fmt.Fprintf(stderr, "error running gain: %v\n", err)
			return 1
		}
		return 0

	case "setup":
		if err := initcmd.Run(cmdArgs); err != nil {
			fmt.Fprintf(stderr, "setup failed: %v\n", err)
			return 1
		}
		return 0

	case "predict":
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		diff := ""
		if len(cmdArgs) > 0 {
			diff = cmdArgs[0]
		} else {
			out, err := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0").Output()
			if err != nil {
				fmt.Fprintf(stderr, "error getting git diff: %v\n", err)
				return 1
			}
			diff = string(out)
		}

		ie := engine.NewImpactEngine(db, nil, nil)
		start := time.Now()
		results, err := ie.PredictTests(ctx, diff)
		if err != nil {
			fmt.Fprintf(stderr, "prediction error: %v\n", err)
			return 1
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
			// predict saves the LLM from having to analyze the full call graph to connect diff with tests.
			// Baseline saving is 4000 tokens plus 500 per affected test predicted.
			savedTokens := 4000 + len(results)*500
			outputTokens := utils.EstimateTokens(outputStr)
			_ = tracker.Track(ctx, "scouter predict", "scouter predict", savedTokens, outputTokens, duration)
		}

		fmt.Fprint(stdout, outputStr)
		return 0
	
	default:
		// Initialize heavy dependencies only for Pipeline routing
		tracker.WarmUp(ctx)
		filters, err := filter.LoadAll(cfg.Filters.Dir)
		if err != nil {
			slog.Warn("could not load filters", "path", cfg.Filters.Dir, "error", err)
		}
		reg := filter.NewRegistry(filters)

		p := &engine.Pipeline{
			Registry:     reg,
			Tracker:      tracker,
			Verbose:      flags.Verbose,
			UltraCompact: flags.UltraCompact,
			Enrich:       flags.Enrich,
		}
		return p.Run(ctx, cmd, cmdArgs)
	}
}
