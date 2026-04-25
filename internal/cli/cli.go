package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
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

	if len(args) < 2 {
		app.printUsage()
		return 0
	}

	cmd := args[1]
	cmdArgs := args[2:]

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
		
		transport := &sdk.StdioTransport{}
		if err := server.Start(ctx, transport); err != nil {
			fmt.Fprintf(stderr, "MCP Server stopped: %v\n", err)
		}
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
	
	default:
		// Fallback to pipeline
		p := &engine.Pipeline{}
		return p.Passthrough(ctx, cmd, cmdArgs)
	}
}
