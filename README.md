# Scouter: Structural Intelligence Engine

**Structural Analysis & Reconnaissance for AI Agents**
Deep AST inspection, Impact analysis, and Automated healing.

Scouter is the tactical visor for your AI coding agents. While memory systems provide context, Scouter provides the structural eyes. Instead of parsing plain text, Scouter parses and navigates the Abstract Syntax Tree (AST), bringing precision to autonomous code operations.

## Overview

Scouter uses Tree-sitter for robust code parsing, indexes structures into SQLite and Bleve, and provides structural intelligence via the Model Context Protocol (MCP). It equips AI agents with 25+ specialized tools to safely navigate, analyze, and modify complex codebases.

### Core Capabilities

- **AST Mapping & Inspection**: Surgical analysis via `ast_map`, `ast_read`, `ast_search`, and `ast_neighborhood` to minimize context noise.
- **Impact Analysis (Blast Radius)**: Calculate exactly which files and symbols will break before modifying the code using `risk_impact` and `risk_predict`.
- **Autonomous Healing**: Automated Root Cause Analysis (RCA) that fixes tests and verifies the solution in a closed loop (`scouter_heal`).
- **Ripple Protocol**: Atomic, multi-file symbol refactoring that maintains structural integrity (`ledger_ripple`, `ledger_evolve`).
- **Staging Ledger**: All mutations are staged in memory and strictly validated before they are committed to disk (`ledger_commit`, `ledger_rollback`, `ledger_diff`).

## Installation

```bash
go install github.com/Rogercode97/scouter/cmd/scouter@latest
```

## Agent Integration

Scouter is built to serve directly into agent workflows.

| Agent | Configuration |
| --- | --- |
| Gemini CLI | `scouter setup gemini-cli` |
| Claude Code | `claude plugin install $(which scouter)` |
| Cursor / VS Code | Add MCP: `{"command": "scouter", "args": ["mcp"]}` |

## Usage Guide

### Command Line Interface

You can execute structural tasks directly via the CLI:

```bash
# Map the codebase and index AST symbols
scouter index .

# Predict affected tests from git diff using the Call Graph
scouter predict

# Map runtime telemetry to AST symbols
cat telemetry.jsonl | scouter ingest --env production
```

### Model Context Protocol (Reference)

Agents access Scouter through specialized MCP tools categorized by their objective.

**Mapping and Verification**
- `ast_map`: Map a file or directory to return its skeleton (signatures without bodies).
- `ast_index`: Index a file or directory for AST symbols.
- `ast_snapshot` / `ast_verify`: Take snapshots to guarantee structural integrity pre and post-edit.

**Navigation and Intelligence**
- `ast_search`: Semantic or text search for symbols.
- `ast_read`: Read specific symbols or fragments.
- `ast_callers`: Trace call hierarchy.
- `ast_definition`: LSP-powered Go-To-Definition.
- `ast_neighborhood`: Get 1-hop structural neighborhood in ZON format.

**Risk and Predictive Analytics**
- `risk_impact`: Calculate the blast radius of a change.
- `risk_predict`: Identify tests affected by current changes.
- `risk_critical_code`: Identify high-risk symbols based on high centrality and fragility.

**Safe Mutation (The Ledger)**
- `ledger_diff`: Review staged changes.
- `ledger_commit`: Atomic commit of staged changes to disk.
- `ledger_rollback`: Clear all staged changes safely.
- `ledger_ripple`: Rename or change signatures across the codebase atomically.
- `ledger_evolve`: Apply multi-file architectural changes safely.

**Cognitive & Diagnostic Engines**
- `scouter_diagnose`: Generate a Diagnostic HUD from error logs.
- `scouter_heal`: Execute a full RCA, Fix, and Verify loop for tests.
- `cognitive_dream`: Distill Architectural Decision Records (ADRs) and pattern summaries.

## Architecture

Scouter relies on a strict internal modular structure:

- `cmd/scouter/`: The CLI application and entry points.
- `internal/mcp/`: The MCP server, handler logic, and tool registrations.
- `internal/engine/`: The core analytical brains, handling search, impact, refactoring, and automated healing.
- `internal/store/`: The data persistence layer (SQLite, Bleve).
- `openspec/`: Architectural specifications and conventions.

**Core Invariant**: Validation is finality. No destructive mutation is ever applied directly to disk without first passing through the Staging Ledger and surviving a comprehensive Impact Analysis.

## Documentation Resources

- **MCP Reference**: `docs/MCP-REFERENCE.md`
- **Architecture Guidelines**: `docs/ARCHITECTURE.md`
- **Codebase Map**: `docs/CODEBASE-GUIDE.md`
- **Security Policy**: `SECURITY.md`
- **Contributing Guidelines**: `CONTRIBUTING.md`

## License

MIT License.
