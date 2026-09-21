<p align="center">
  <img src="assets/scouter-banner-v4.jpg" alt="Scouter: Structural Intelligence Engine" width="100%" />
</p>

# Scouter: Structural Intelligence Engine

**Structural Analysis & Reconnaissance for AI Agents**
Deep AST inspection, impact analysis, and safe atomic mutation.

Scouter is the tactical visor for your AI coding agents. While memory systems provide context, Scouter provides the structural eyes. Instead of parsing plain text, Scouter parses and navigates the Abstract Syntax Tree (AST), bringing precision to autonomous code operations.

## Overview

Scouter is a high-performance, CGO-free, **single-binary CLI engine**. It uses Tree-sitter for code parsing, indexes structures into SQLite (WASM sandbox) and Bleve, and answers structural questions on demand: blast radius, symbol centrality, git churn risk, and safe multi-file mutation through a Staging Ledger.

It runs strictly as an **on-demand CLI**: no background daemons, no file watchers. This keeps memory flat and preserves inotify handles on constrained devices such as Termux. Complementing it, [CodeGraph](https://github.com/colbymchenry/codegraph) acts as the fast knowledge-graph reader for symbol exploration.

A Model Context Protocol server is also shipped for agents that prefer a tool-call surface over shelling out. The CLI remains the primary and fully supported interface.

### Core Capabilities

- **AST Mapping & Inspection**: Surgical analysis with `context` (token-dense architectural packet for a file), `neighborhood` (1-hop structural surroundings), and `twins` (logical duplication).
- **Impact Analysis (Blast Radius)**: Calculate exactly which files and symbols will break before modifying code with `impact` and `predict`.
- **Symbol Centrality**: Rank symbols by reference centrality with `critical`, and export the call graph as Mermaid with `graph`.
- **Safe Mutation (The Ledger)**: Every mutation is staged in memory and validated before it reaches disk via `diff`, `commit`, and `rollback`.
- **Diagnostic & Healing**: `audit`, `fix`, and `flow` drive root-cause analysis and closed-loop repair.

## Installation

```bash
go install github.com/Rogercode97/scouter/cmd/scouter@latest
```

## Agent Integration

Scouter can inject its MCP configuration directly into an agent's settings:

```bash
scouter setup <agent>
```

| Agent | Command |
| --- | --- |
| Gemini CLI | `scouter setup gemini-cli` |
| Antigravity CLI | `scouter setup antigravity-cli` |
| OpenCode | `scouter setup opencode` |
| Codex | `scouter setup codex` |
| Cursor | `scouter setup cursor` |
| Windsurf | `scouter setup windsurf` |
| Claude Code | `scouter setup claude` |

*(For full manual configuration and advanced options, see the [Agent Setup Guide](docs/tutorials/AGENT-SETUP.md))*

## Usage Guide

### Command Line Interface

The CLI is the primary surface. All commands are fast, on-demand, and safe to invoke per call.

```bash
# Map the codebase and index AST symbols
scouter index .

# Token-dense architectural context for one file: LOC, symbols, risk, churn, edit cost
scouter context internal/engine/analyzer.go

# 1-hop structural neighborhood of a file
scouter neighborhood internal/engine/analyzer.go

# Call graph for one symbol, exported as Mermaid (callers + callees)
scouter graph GetNeighborhood

# Highest-centrality symbols (hotspots)
scouter critical 20

# Blast radius before touching a symbol
scouter impact

# Predict affected tests from the current git diff
scouter predict

# Review staged changes, then commit or roll back atomically
scouter diff
scouter commit
scouter rollback

# Map runtime telemetry to AST symbols
cat telemetry.jsonl | scouter ingest --env production
```

Add the global `-u` / `--ultra-compact` flag to maximize context efficiency when piping output into an agent:

```bash
scouter -u context internal/engine/analyzer.go
```

### Model Context Protocol (Reference)

For agents wired through `scouter setup`, the MCP server exposes 34 tools grouped by objective. These are the MCP tool names; the CLI commands above are the equivalent on-demand surface.

**Mapping and Verification**
- `ast_map`: Map a file or directory to return its skeleton (signatures without bodies).
- `ast_index`: Index a file or directory for AST symbols.
- `ast_snapshot` / `ast_verify`: Take snapshots to guarantee structural integrity pre and post-edit.

**Navigation and Intelligence**
- `ast_search`: Semantic or text search for symbols.
- `ast_read`: Read specific symbols or fragments.
- `ast_callers`: Trace call hierarchy.
- `ast_definition`: LSP-powered go-to-definition.
- `ast_type_info`: Type information for a symbol.
- `ast_neighborhood`: Get 1-hop structural neighborhood in ZON format.
- `ast_structural_search` / `ast_provenance`: Structural pattern matching and change provenance.

**Risk and Predictive Analytics**
- `risk_impact`: Calculate the blast radius of a change.
- `risk_predict`: Identify tests affected by current changes.
- `risk_critical_code`: Identify high-risk symbols based on high centrality and fragility.
- `risk_lint_architecture` / `risk_semantic_diff`: Architectural lint and semantic diffing.

**Safe Mutation (The Ledger)**
- `ledger_diff`: Review staged changes.
- `ledger_commit`: Atomic commit of staged changes to disk.
- `ledger_rollback`: Clear all staged changes safely.
- `ledger_ripple`: Rename or change signatures across the codebase atomically.
- `ledger_evolve`: Apply multi-file architectural changes safely.

**Cognitive & Diagnostic Engines**
- `scouter_diagnose`: Generate a Diagnostic HUD from error logs.
- `scouter_heal`: Execute a full RCA, fix, and verify loop for tests.
- `cognitive_dream`: Distill Architectural Decision Records (ADRs) and pattern summaries.
- `cognitive_anchor` / `cognitive_compact` / `cognitive_obsidian` / `cognitive_signal`: Context anchoring, compaction, and signal extraction.
- `scouter_unlock`: Recover an interrupted ledger session.

## Architecture

Scouter relies on a strict internal modular structure:

- `cmd/scouter/scoutercmd/`: The modular CLI command definitions (Cobra).
- `cmd/scouter/`: The application entry point (`main.go`).
- `internal/engine/`: The core analytical brains, handling search, impact, refactoring, and automated healing.
- `internal/store/`: The data persistence layer (SQLite in a WASM sandbox, Bleve).
- `internal/mcp/`: The MCP server, handler logic, and tool registrations.
- `openspec/`: Architectural specifications and conventions.

**Core Invariant**: Validation is finality. No destructive mutation is ever applied directly to disk without first passing through the Staging Ledger and surviving a comprehensive Impact Analysis.

## Documentation Resources

- **Architecture Overview**: `docs/architecture/overview.md`
- **Engine Internals**: `docs/architecture/engine.md`
- **Codebase Guide**: `docs/architecture/codebase.md`
- **MCP Reference**: `docs/reference/MCP-REFERENCE.md`
- **Decisions (ADRs)**: `docs/adr/`
- **Security Policy**: `SECURITY.md`
- **Contributing Guidelines**: `CONTRIBUTING.md`

## License

MIT License.
