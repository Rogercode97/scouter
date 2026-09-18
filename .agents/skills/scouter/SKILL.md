---
name: scouter
description: "Trigger: 'scouter', 'ast map'. Map exhaustively - AST-based project indexing and structural analysis."
wave: 17.0
dispatch_intent: "Sovereign protocol for AST-based project indexing and structural analysis."
formatter: sovereign
version: 5
---

> **CORE SOVEREIGN LAWS**:
> `SKILL: Load ../../_shared/sovereign-constitution.md`
> `SKILL: Load ../../_shared/resilience-poka-yoke.md`
> `SKILL: Load ../../_shared/engram-protocol.md`


## Outcome Contract

Outcome: Execute strict operations as defined by the protocol.
Done when: Validation tests pass and criteria are met.
Evidence: CLI output (`git diff`, `go test`, `npm test`, etc.).
Output: Verified code artifacts.


# 🗺️ Scouter Dominion (AST Code Graph)

## 🛡️ CRITICAL RULES (Start Here - High Attention Zone)

1. **AST over grep**: Text is a projection; the AST is the truth. Use `search` and `callers`, never grep. ➔ CRITERIA: [git diff/grep verified]
2. **Impact before refactor**: ALWAYS run `impact` to map blast radius BEFORE any refactor. ➔ CRITERIA: [git diff/grep verified]
3. **Ledger mutations**: Structural changes MUST be reviewed via `scouter_diff` before `scouter_commit`. Direct destructive mutation is forbidden. ➔ CRITERIA: [git diff/grep verified]
4. **Bounded retrieval**: Respect AST depth limits. Rely on pre-materialized CTE bounds to prevent context explosion. ➔ CRITERIA: [git diff/grep verified]

## Workflow (Middle - Minimal)

| Task | Tool / Command | Context |
|---|---|---|
| Search/Symbols | `scouter search <q>` (or `codegraph explore`) | AST symbol lookup |
| Context Packet | `scouter context <file> [-u]` | Single-shot agent context (dependents, blast radius, edit cost) |
| Structural Maps | `codegraph callers`, `codegraph callees` | Deep knowledge graph traversal |
| Blast Radius | `scouter impact <sym>` | Map recursive CTE dependencies |
| Hotspots / Churn | `scouter critical` | PageRank + Git Churn risk |
| Indexing | `scouter index .` | Fast structural SQLite sync |
| Ledger (2PC) | `scouter diff`, `commit`, `rollback` | Atomic mutations before disk |

## ✅ OUTPUT (End - High Attention Zone)

```json
{
  "skill_name": "scouter",
  "operation": "search | impact | index | commit",
  "blast_radius": "low | medium | high",
  "engram_sync": true,
  "next_logical_skill": "sdd-apply | dsr/codebase-research"
}
```

**Done when**: Blast radius mapped, diff reviewed, findings documented, next skill identified.


# AST and Structural Search Brief (Scouter Dominion)

## 1. 🛡️ The Structural Mandate
- **AST Over Grep**: Text is a projection; the AST is the truth. Use structural patterns instead of text searches for functions, variables, or logic to eliminate false positives.
- **Regex Prohibition**: NEVER use Regular Expressions to parse or modify nested logic or complex function signatures. Regular expressions are incapable of parsing context-free grammars (like nested brackets).
- **Surgical Boundaries**: Apply refactors based on AST node coordinates (line/char from Scouter), not arbitrary line numbers.
- **Mathematical Soundness**: By operating on the Abstract Syntax Tree, we eliminate the false positives and fragility inherent in text-based searches (`grep`).

## 2. ⚙️ Structural Operations
- **Find Definitions/Usages**: Use `scouter search <query>` or `codegraph explore <symbol>` / `codegraph node <symbol>`.
- **Verify Consistency**: Always re-run the structural query after changes to verify zero residues of the old pattern.
- **AST Coordinates**: Rely on the precise coordinates provided by `scouter` to target changes surgically.

# Graph Analysis and Impact Brief (Scouter Dominion)

## 1. 🛡️ Graph Mandates
- **Deterministic Graph**: Trust the code graph; never assume file proximity implies logical dependency.
- **Call Graph Over File Path**: Assumptions about file structure are often wrong. Only the call graph provides the deterministic truth of a symbol's usage.
- **Self-Healing**: Always verify call graph consistency immediately after applying structural changes.

## 2. ⚙️ Execution & Traversal
- **Trace Callers**: Use `codegraph callers <symbol>` or `scouter graph`.
- **Blast Radius**: Always run `scouter impact <symbol>` to determine the blast radius BEFORE modifying any symbol. We don't guess the impact of a change; we calculate it (transitive closure of all callers).
- **Ripple Engine**: Traversing the dependency graph starting from "Impacted Symbols" using `BFSPropagationStrategy`.

## 3. 🚦 Decision Gates
- **Calculate Blast Radius**: Run `scouter impact <symbol>` to identify all affected sites.
- **Verify Consistency**: Use the analyzer for post-indexing resolution of relationships (e.g., "Struct A implements Interface B").

# Indexing and Hotspots Brief (Scouter Dominion)

## 1. 🛡️ Index Integrity Mandates
- **Index Integrity**: Always run `scouter index .` before calculating impact if the codebase has changed since the last index.
- **Recursive Refresh**: Trigger a re-index after any structural change (rename, move, or delete).

## 2. ⚙️ Execution & Metrics
- **PageRank Weighting**: Rank symbols by their centrality (frequency of reference). Consult centrality metrics before refactoring high-weight symbols.
- **Identify Hotspots**: Run `scouter critical` to identify symbols with high centrality or fragility (Git churn).
- **Fresh Project Scan**: Execute `scouter index .` at the start of a session or after significant external changes.

## 3. 🚦 Decision Gates
- **Identify Hotspots**: Run `scouter critical`.
- **High-volume filtering**: RTK. Pipe `scouter` output with `-u` / `--ultra-compact`.

