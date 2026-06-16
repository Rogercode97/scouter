package cli

import (
	"fmt"
	"io"
)

var Version = "dev"

type App struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (app *App) printUsage() {
	usage := `scouter v%s — CLI Token Killer & Oracle Engine

Usage:
  scouter <command> [arguments]

  Core Commands:
    propagate <sym> <trans> Stage a semantic refactoring
    commit                  Apply staged changes in ledger
    rollback                Discard staged changes in ledger
    diff                    Show staged changes diff
    status                  Show ledger status
    index <path>    Index a file or directory for structural intelligence (--deep for Go SSA)
    search <query>   Search for symbols across AST and historical insights
    flow <symbol>    Trace the origin of a variable or symbol
    graph [filter]   Export the Call Graph in Mermaid format
    predict [diff]  Identify tests affected by current changes
    impact <symbol> <path> Analyze architectural impact of changing a symbol
    audit [path]    Run architectural rules audit on a directory
    fix <log_file>  Diagnose and propose fixes for an error log
    neighborhood <file>  Get 1-hop structural context (imports, exports, calls) in ZON format
    twins <symbol> <path> Find structurally identical duplicate functions
    critical [limit]      List top highly connected symbols in the Call Graph
    setup           Interactive environment configuration
    gain [range]    Display token savings and ROI metrics
    mcp             Start the Model Context Protocol (MCP) server
    ingest          Process external logs for passive health tracking

Options:
  -v, --verbose   Enable detailed logging
  --ultra-compact Maximize context efficiency in output
  --enrich        Enable deep AST enrichment for proxied commands
`
	fmt.Fprintf(app.Stdout, usage, Version)
}
