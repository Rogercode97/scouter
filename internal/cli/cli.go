package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/initcmd"
	"github.com/Rogercode97/scouter/internal/mcp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/telemetry"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "2.6.2-sovereign"

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

	switch cmd {
	case "mcp":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
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
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
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
		te := engine.NewTruthEngine(db, analyzer, lspMgr, nil, nil, nil, nil, nil, nil)
		
		if err := te.Index(ctx, path); err != nil {
			fmt.Fprintf(stderr, "index error: %v\n", err)
			return 1
		}

		fmt.Printf("✅ Indexed %s\n", path)
		return 0

	case "gain":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
		tracker, err := telemetry.NewTracker(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error creating tracker: %v\n", err)
			return 1
		}
		defer tracker.Close()
		// display.RunGain also writes to stdout, this part is still not fully testable
		// but we accept this for now.
		return 0

	case "setup":
		if err := initcmd.Run(cmdArgs); err != nil {
			fmt.Fprintf(stderr, "setup failed: %v\n", err)
			return 1
		}
		return 0

	case "predict":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
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

		ie := engine.NewImpactEngine(db, nil)
		results, err := ie.PredictTests(ctx, diff)
		if err != nil {
			fmt.Fprintf(stderr, "prediction error: %v\n", err)
			return 1
		}

		if len(results) == 0 {
			fmt.Fprintln(stdout, "No affected tests identified.")
			return 0
		}

		fmt.Fprintln(stdout, "🎯 Affected Tests:")
		for _, r := range results {
			fmt.Fprintf(stdout, "- %s (%s)\n", r.Name, r.File)
		}
		return 0
	
	default:
		// Fallback to pipeline
		p := &engine.Pipeline{
			Verbose:      flags.Verbose,
			UltraCompact: flags.UltraCompact,
			Enrich:       flags.Enrich,
		}
		return p.Passthrough(ctx, cmd, cmdArgs)
	}
}
