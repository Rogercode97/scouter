# Scouter MCP Tools & Engram Reference

This document provides a factual reference for Scouter's Model Context Protocol (MCP) tools, with a specific focus on their integration with the **Engram Persistence Engine**.

## Architectural Overview: The Engram Synergy

Scouter's MCP server is not a stateless proxy. It leverages **Engram** (a SQLite-based persistent memory provider) to maintain cross-session context, track historical bugfixes, and distill long-term architectural lore.

### Core Persistence Points
- **Knowledge Capture**: Storing session summaries as "Anchors".
- **Context Injection**: Informing autonomous healing and adversarial reviews with past decisions (ADRs).
- **Metric Enrichment**: Calculating risk based on historical churn and bugfix frequency.

---

## 1. Cognitive Tools (Memory & Lore)

These tools directly manage or utilize the Engram memory layer.

### `cognitive_anchor`
**Purpose**: SESSION PERSISTENCE. Saves a high-density technical summary of the current session into Engram.
- **Parameters**:
  - `summary` (string, **REQUIRED**): The technical summary to persist.
- **Engram Interaction**: Creates a new `session_summary` observation in the project's SQLite database.

### `cognitive_dream`
**Purpose**: ARCHITECTURAL ALIGNMENT. Triggers the memory distillation loop.
- **Parameters**:
  - `project` (string, optional): Name of the project (defaults to current repo).
  - `hours` (int, optional): Lookback window for extraction (default: 24).
- **Engram Interaction**: Reads recent raw observations and session history to generate ADRs (Architectural Decision Records) and Pattern summaries.

### `cognitive_compact`
**Purpose**: CONTEXT MANAGEMENT. Optimizes the LLM's context window.
- **Parameters**:
  - `force` (bool, optional): Force compaction.
- **Engram Interaction**: Uses critical context from previous sessions to ensure high-fidelity summarization.

---

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
- [Architecture Overview](ARCHITECTURE.md) — Deep dive into the TruthEngine and Engram adapter.
- [Sovereign Directives](server.go) — Internal descriptions and AI-First constraints.
