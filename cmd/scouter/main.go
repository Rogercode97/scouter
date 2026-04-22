package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/initcmd"
	"github.com/Rogercode97/scouter/internal/mcp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/telemetry"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "init", "setup":
		if err := initcmd.Run(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
	case "mcp":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize store: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()
		
		server := mcp.NewServer(db)
		if err := server.Run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "MCP Server stopped: %v\n", err)
		}
	case "gain":
		tracker, _ := telemetry.NewTracker(ctx, "local")
		display.RunGain(tracker, os.Args[2:])
	case "config":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Config loaded from: %s\n", cfg.Tracking.DBPath)
	case "proxy":
		engine.Passthrough(ctx, os.Args[2], os.Args[3:])
	case "--version":
		fmt.Println("scouter v2.6.0")
	case "--help":
		printUsage()
	default:
		result, err := engine.Execute(ctx, os.Args[1], os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Print(result.Stdout)
		fmt.Fprint(os.Stderr, result.Stderr)
	}
}

func printUsage() {
	fmt.Println("Scouter — Code analysis engine for AI agents")
	fmt.Println("\nUsage:")
	fmt.Println("  scouter mcp            Start MCP server (stdio transport)")
	fmt.Println("  scouter setup <agent>  Install scouter integration (gemini-cli, opencode)")
	fmt.Println("  scouter version        Show version")
}
