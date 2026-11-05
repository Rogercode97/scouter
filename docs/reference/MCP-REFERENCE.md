# Scouter MCP Tools & Engram Reference

This document provides a factual reference for Scouter's Model Context Protocol (MCP) tools, with a specific focus on their integration with the **Engram Persistence Engine**.

## Architectural Overview: The Engram Read Path

The MCP server reads Engram (a SQLite observation database) as an optional
source of historical bugfix insights. It never writes to that database.

- **Metric Enrichment**: `impact` and `risk_impact` multiply a symbol's risk by
  `1.0 + 0.2 x historical_bugfixes`, so a symbol with a bugfix history scores
  higher than an identical symbol without one.
- **Degradation**: when the database is absent, `impact` answers without the
  multiplier and no error. Nothing is inferred or synthesized in its place.

Session persistence and distillation are gone: `cognitive_anchor` and
`cognitive_dream` were removed with the `internal/adapters` prune.

## 1. Cognitive Tools (Signal & Capture)

The `cognitive_compact` summary is a **prompt**, not a tool; see Prompts below.

### `cognitive_signal`
**Purpose**: NOISE FILTER. Reduces raw text to its signal.
- **Parameters**:
  - `text` (string, **REQUIRED**): Text to filter.
  - `mode` (string, optional): Filtering mode.
  - `level` (string, optional): Aggressiveness (default: `aggressive`).

### `cognitive_obsidian`
**Purpose**: Export an impact analysis as an Obsidian-ready note.
- **Parameters**:
  - `symbolName` (string, **REQUIRED**)
  - `filePath` (string, **REQUIRED**)
  - `vaultPath` (string, optional): Custom vault export path.

## 2. Risk & Impact Tools (Data-Driven Insight)

These tools use Engram data to quantify risk and predict blast radius.

### `risk_impact`
**Purpose**: CRITICAL IMPACT ANALYSIS. Calculates the blast radius of a change.
- **Parameters**:
  - `symbolName` (string, **REQUIRED**): Symbol to analyze.
  - `filePath` (string, **REQUIRED**): Path to the file.
- **Engram Interaction**: Pulls historical bugfix counts and churn metrics to calculate the final `RiskScore` (0.0 - 1.0).

### `risk_critical_code`
**Purpose**: RISK IDENTIFICATION. Finds the most fragile parts of the system.
- **Engram Interaction**: Ranks symbols based on a fusion of PageRank (structural centrality) and Engram Churn (historical instability).

---

## 3. Specialized Engines (Autonomous Reasoning)

Tools that use Engram to inform complex decision-making.

### `scouter_heal`
**Purpose**: AUTONOMOUS HEALING. Fixes test failures using the Shinigami Protocol.
- **Engram Interaction**: Fetches historical bugfixes and related RCA (Root Cause Analysis) from Engram to guide the `DoFixRequest` sampling.

### `scouter_judge`
**Purpose**: ADVERSARIAL REVIEW. Audits a proposal using parallel judges.
- **Engram Interaction**: Injects relevant ADRs from Engram into the system prompt to ensure the "Judges" are aligned with past architectural decisions.

### `scouter_sdd`
**Purpose**: SDD RADAR. Navigates Source Driven Development artifacts.
- **Engram Interaction**: Queries the project roadmap, tasks, and specs stored in the specialized SDD memory tables.

---

## 4. AST & Ledger Tools (Structural Integrity)

While these tools are primarily AST-based, they provide the "Pure Signal" that eventually populates Engram.

| Tool Name | Action | Logic |
| :--- | :--- | :--- |
| `ast_map` | Skeleton Mapping | Returns signatures without bodies (Token Optimization). |
| `ast_snapshot` | Structural Guard | Takes a snapshot before editing. |
| `ledger_commit` | Finality | Persists staged changes to disk. |
| `ledger_ripple` | Propagation | Renames symbols across the entire codebase. |

---

## Related Documentation
- [Architecture Overview](../architecture/overview.md) — Deep dive into the Engine Architecture and Engram adapter.
- [Sovereign Directives](server.go) — Internal descriptions and AI-First constraints.
