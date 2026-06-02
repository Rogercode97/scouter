package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/domain/memory"
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

Usage:
  scouter <command> [arguments]

Core Commands:
  index <path>    Index a file or directory for structural intelligence (--deep for Go SSA)
  search <query>   Search for symbols across AST and historical insights
  flow <symbol>    Trace the origin of a variable or symbol
  graph [filter]   Export the Call Graph in Mermaid format
  predict [diff]  Identify tests affected by current changes
  setup           Interactive environment configuration
  gain [range]    Display token savings and ROI metrics
  mcp             Start the Model Context Protocol (MCP) server
  ingest          Process external logs for passive health tracking

Options:
  -v, --verbose   Enable detailed logging
  --ultra-compact Maximize context efficiency in output
  --enrich        Enable deep AST enrichment for proxied commands
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

	// Database: Centralized initialization for commands that need it
	openDB := func() (store.Store, func(), int) {
		db, err := store.NewStore(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return nil, nil, 1
		}
		return db, func() { db.Close() }, 0
	}

	// Telemetry: Lazy tracker to avoid SQLite locks on fast paths
	tracker := telemetry.NewLazyTracker(cfg.Tracking.DBPath)
	defer tracker.Close()

	switch cmd {
	case "mcp":
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		
		engramPath, _ := engram.DiscoverDBPath()
		memoryProvider := engram.NewSQLiteMemoryProvider(engramPath)

		lspMgr := lsp.GetGlobalManager()
		ledger := engine.NewLedger()
		impact := engine.NewImpactEngine(db, lspMgr, memoryProvider)
		analyzer := engine.NewAnalysisEngine(db)
		ripple := engine.NewRippleEngine(db, nil, impact)
		ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))
		searchEngine := engine.NewSearchEngine(db, memoryProvider)
		healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, memoryProvider)
		compact := engine.NewCompactionEngine(db, ledger)
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, healer, lspMgr)
		sdd := engine.NewSDDEngine(".")

		truthEngine := engine.NewTruthEngine(
			db,
			engine.WithMemory(memoryProvider),
			engine.WithAnalyzer(analyzer),
			engine.WithLSP(lspMgr),
			engine.WithImpact(impact),
			engine.WithSearch(searchEngine),
			engine.WithCompact(compact),
			engine.WithHealer(healer),
			engine.WithDiagnostic(diagnostic),
			engine.WithRipple(ripple),
			engine.WithSDD(sdd),
			engine.WithLedger(ledger),
		)

		appService := memory.NewAppService(memoryProvider, nil)
		chronos := engine.NewChronosEngine()

		opts := mcp.Options{
			Store:         db,
			Logger:        logger,
			LspMgr:        lspMgr,
			TruthEngine:   truthEngine,
			ChronosEngine: chronos,
			AppService:    appService,
		}

		server := mcp.NewServer(opts)
		defer server.Close()
		
		transport := &sdk.StdioTransport{}
		if err := server.Start(ctx, transport); err != nil {
			fmt.Fprintf(stderr, "MCP Server stopped: %v\n", err)
		}
		return 0

	case "index":
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

		if len(cmdArgs) == 0 {
			fmt.Fprintf(stderr, "usage: scouter index <path> [--deep]\n")
			return 1
		}

		path := cmdArgs[0]
		lspMgr := lsp.GetGlobalManager()
		defer lspMgr.Close()
		
		analyzer := engine.NewAnalysisEngine(db)
		te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer), engine.WithLSP(lspMgr))
		
		start := time.Now()
		if err := te.Index(ctx, path); err != nil {
			fmt.Fprintf(stderr, "index error: %v\n", err)
			return 1
		}

		// Mode Deep: High-Precision SSA Analysis (Go only)
		if flags.Deep {
			fmt.Fprintf(stdout, "🛡️  Running Mode Deep (SSA Analysis)...\n")
			
			// Ensure we have a directory for SSA loading
			absPath, _ := filepath.Abs(path)
			info, err := os.Stat(absPath)
			if err == nil && !info.IsDir() {
				absPath = filepath.Dir(absPath)
			}
			
			ssaCalls, err := engine.SSACallGraph(ctx, absPath)
			if err != nil {
				fmt.Fprintf(stderr, "SSA analysis error: %v\n", err)
				// Don't fail the whole index if SSA fails (it's a best-effort deep dive)
			} else {
				for _, c := range ssaCalls {
					_ = db.SaveCall(ctx, store.Call{
						CallerName: c.CallerName,
						CalleeName: c.CalleeName,
						LinkType:   c.LinkType,
						Path:       c.Path,
						Line:       c.Line,
					})
				}
				fmt.Fprintf(stdout, "✨ Deep Analysis complete (%d high-precision calls found)\n", len(ssaCalls))
			}
		}

		duration := time.Since(start).Milliseconds()

		fc, sc, err := db.GetStats(ctx)
		if err == nil && tracker != nil {
			savedTokens := fc*1500 + sc*100
			_ = tracker.Track(ctx, "scouter index "+path, "scouter index "+path, savedTokens, 5, duration)
		}

		fmt.Printf("✅ Indexed %s\n", path)
		return 0

	case "graph":
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

		var calls []store.Call
		filter := ""
		if len(cmdArgs) > 0 {
			filter = cmdArgs[0]
			results, err := db.GetCallees(ctx, filter)
			if err != nil {
				fmt.Fprintf(stderr, "error fetching callees: %v\n", err)
				return 1
			}
			calls = results

			// Also get callers to show the immediate context
			callers, _ := db.GetCallers(ctx, filter, 50, 0)
			calls = append(calls, callers...)
		} else {
			// Get all calls (limited to prevent huge graphs)
			seq := db.GetAllCalls(ctx)
			count := 0
			for call, err := range seq {
				if err != nil {
					break
				}
				calls = append(calls, call)
				count++
				if count > 500 { // Safety limit
					break
				}
			}
		}

		title := "Scouter Call Graph"
		if filter != "" {
			title = fmt.Sprintf("Graph for %s", filter)
		}

		mermaid := display.ExportMermaid(calls, title)
		fmt.Fprintln(stdout, mermaid)
		return 0

	case "search":
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

		if len(cmdArgs) == 0 {
			fmt.Fprintf(stderr, "usage: scouter search <query>\n")
			return 1
		}

		query := cmdArgs[0]
		search := engine.NewSearchEngine(db, nil)
		
		results, err := search.HybridSearch(ctx, query, 10, 0)
		if err != nil {
			fmt.Fprintf(stderr, "search error: %v\n", err)
			return 1
		}

		if len(results.Symbols) == 0 {
			fmt.Fprintf(stdout, "No results found for %q\n", query)
			return 0
		}

		fmt.Fprintf(stdout, "🔍 Search results for %q:\n", query)
		for _, sym := range results.Symbols {
			fmt.Fprintf(stdout, "- [%s] %s (%s:%d)\n", sym.Type, sym.Name, sym.Path, sym.StartLine)
		}
		return 0

	case "flow":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(stderr, "usage: scouter flow <symbol>\n")
			return 1
		}
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()
		flows, err := db.GetFlows(ctx, cmdArgs[0])
		if err != nil {
			fmt.Fprintf(stderr, "error fetching flows: %v\n", err)
			return 1
		}
		display.PrintFlows(stdout, cmdArgs[0], flows)
		return 0

	case "ingest":
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

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
		db, closeDB, exitCode := openDB()
		if exitCode != 0 {
			return exitCode
		}
		defer closeDB()

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
