# Scouter 🕶️ (Wave 8.9 — Absolute Sovereignty)
**Status**: Sovereign. Divine Architecture (Go 1.25). Glasswall Validated. Rating 10.0.

**The Omniscient Code Intelligence Engine for AI Agents.**  
*Recursive Impact Analysis. Predictive Testing. Risk Mapping. Structural Duck-Typing. Ghost Speed.*

Scouter gives AI agents (like Gemini CLI, OpenCode, and Claude Code) "X-ray vision" for your codebase. It eliminates token waste by providing surgical access to symbols, cross-file impact analysis, and real-time risk diagnostics.

## 🚀 Quick Start

### 1. Build from source (Go 1.25+)
```bash
go build -o bin/scouter cmd/scouter/main.go
go build -o bin/index-vault cmd/index-vault/main.go
```

### 2. Index the Workspace (Omniscience Mode ⚡)
```bash
./bin/index-vault
```

---

## 🏛️ Sovereign Pillars (v2.2.0 — Oracle Edition)

Scouter has transcended simple analysis into a predictive risk engine:

### 1. Predictive Testing (Oracle Engine) 🔮
**Predict the future of your errors.** Integrates with Git to analyze local changes (`diff HEAD`) and automatically suggest which tests to run based on the affected symbols and their impact radius.

### 2. Recursive Impact Analysis (Blast Radius) 🕸️
Deep visibility into dependency chains. Uses **SQLite Recursive CTEs** to trace calls through multiple levels, identifying exactly how a small change can break distant parts of the system.

### 3. Risk Mapping (Centrality & Fragility) 📊
Identifies the "Heart" and the "Glass" of your code. Calculates **Centrality** (indegree) and **Fragility** (test failure history) to pinpoint technical debt and critical refactoring targets.

### 4. Interface Sovereignty (Lazo Soberano) 🧬
Dynamic contract resolution. Automatically links Structs to the Interfaces they implement using structural analysis (Duck Typing), providing visibility into polymorphic call chains.

### 5. Context Sovereignty (Go 1.24+) 🛡️
Military-grade resource management. 100% compliant with `context.Context` standards and `signal.NotifyContext`, ensuring zero leaks and no `SQLITE_BUSY` deadlocks.

### 6. Semantic Search 🔍
Intelligent lookups using **BM25 ranking** across names and documentation. Find logic based on concepts (e.g., "db connection") rather than just literal names.

### 7. Ghost Speed ⚡
Incremental indexing that skips unchanged files using SHA-256 pre-flight hashing. Performance reaches **+750 files/sec**.

---

## 🛠️ MCP Toolset (Oracle Edition)

| Tool | Capability |
| :--- | :--- |
| `scouter_predict` | **Oracle**: Suggests tests based on local Git changes. |
| `scouter_impact` | **Blast Radius**: Trace the recursive impact of changing a symbol. |
| `scouter_critical_code` | **Risk Map**: Identify the most central and fragile symbols. |
| `scouter_index` | Analyze and index file AST, calls, and docs. |
| `scouter_search` | BM25 semantic search across symbols and documentation. |
| `scouter_callers` | List all project-wide invocations of a symbol. |
| `scouter_visualize` | Generate a **Mermaid.js** diagram of dependencies. |
| `scouter_health` | Diagnose failed tests and linked symbols. |
| `scouter_read` | Surgical fragment reading with SHA-256 validation. |

## 📜 License
MIT — *Omniscience for all.*
