# Getting Started with Scouter

Scouter is a CLI Token Killer & Oracle Engine designed for structural intelligence and context-aware codebase analysis.

## Core Capabilities

Scouter provides a set of powerful commands to understand and analyze your codebase structurally:

- **`scouter index <path>`**: Index a file or directory. Use `--deep` for Go SSA analysis.
- **`scouter search <query>`**: Search for symbols across the AST and historical insights.
- **`scouter flow <symbol>`**: Trace the origin of a variable or symbol.
- **`scouter graph [filter]`**: Export the Call Graph in Mermaid format.
- **`scouter predict [diff]`**: Identify tests affected by current changes based on structural impact.

## Advanced Usage

For deep integrations and AI agent workflows, Scouter supports:
- **`scouter mcp`**: Start the Model Context Protocol (MCP) server.
- **`scouter gain [range]`**: Display token savings and ROI metrics.
- **`scouter ingest`**: Process external logs for passive health tracking.

## Configuration Options
- `-v, --verbose`: Enable detailed logging.
- `--ultra-compact`: Maximize context efficiency in output (highly recommended for LLMs).
- `--enrich`: Enable deep AST enrichment for proxied commands.
