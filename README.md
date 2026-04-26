# Scouter 🕶️ (V2.0 — Omniscience Edition)
**Status**: Sovereign. Divine Architecture (Go 1.25). Rating 10.0.

**The Sovereign Truth Kernel (RTK) & Architectural Oracle for AI Agents.**  
*Predictive Impact Analysis. 0.0-1.0 Risk Scoring. Mermaid Blast Radius. Semantic Omniscience.*

Scouter is more than a tool; it is an **Architectural Oracle** that sits between your codebase and your AI agent. It uses high-performance SQLite Recursive CTEs to map the recursive blast radius of changes, surfacing structural risks before they are committed.

## 🚀 The Divine Synergy (Scouter + RTK)

For the ultimate Hakaishin experience, Scouter (Static Intelligence) and **RTK** (Dynamic Filtering) work together. 

**Activate the synergy in one strike:**
```bash
# 1. Install the Muscle (RTK)
brew install rtk && rtk init -g --gemini

# 2. Build the Brain (Scouter)
make build && ./bin/scouter setup gemini
```

### Why both?
- **RTK (Rust)**: Installs global shell hooks to kill 90% of noise in every command (`git`, `npm`, `go test`).
- **Scouter (Go)**: Provides semantic omniscience (Recursive Impact, Risk Scoring, AST search) and the MCP bridge for Gemini CLI.

---

## 🎯 Omniscience Engine (V2.0)

Scouter V2.0 introduces the **Impact Analysis Engine**, transforming the tool into a predictive oracle:

### 🧠 Predictive Risk Scoring (0.0 - 1.0)
Every entity (function/method) is assigned a dynamic Risk Score based on:
- **Centrality**: Direct dependency count (indegree).
- **Blast Radius**: Size of the transitive dependency tree.
- **Exported Status**: Public API exposure risk.
*Gating criteria: Changes with Risk > 0.8 (Critical) require Divine Review.*

### 📈 Visual Blast Radius (Mermaid.js)
Generate ready-to-render Mermaid.js flowchart code for any impact chain. Visualize how a modification in `internal/utils` ripples through the entire system.

### 🧬 Entity-Level Granularity
Move beyond file-level diffs. Scouter understands precisely which function or method is affected, providing O(1) architectural lookup via optimized B-tree indexes.

---

## 🏗️ Pure Signal Architecture (Wave 9)

Scouter enforces a strict **Signal Isolation** protocol to ensure 100% stability in MCP environments:
- **Sacred Channel**: `os.Stdout` is reserved exclusively for JSON-RPC messages. 
- **Isolated Telemetry**: All internal logs, warnings, and process outputs (via `Passthrough`) are redirected to `os.Stderr`.
- **Atomic Writes**: Every message is protected by a `sync.Mutex` and formatted as single-line JSON with explicit flushing to prevent framing errors.

---

## 🛠️ MCP & CLI Toolset

| Tool | Capability |
| :--- | :--- |
| `scouter_impact` | **Oracle**: Perform recursive impact analysis with 0.0-1.0 Risk Scoring and Mermaid maps. |
| `scouter_pure_signal` | **Synergy**: Pipes any text into RTK's Rust engine for instant purification. |
| `scouter_read` | **Surgical**: Semantic fragment reading with Pointer Resolution. |
| `scouter_search` | **Search**: BM25 semantic search across symbols and docs. |
| `scouter_index` | **Index**: Deep AST indexing for Go, TS, and Python with centrality tracking. |
| `scouter_callers` | **Graph**: Find all callers of a specific symbol (Global Call Graph). |
| `scouter_critical_code` | **Risk**: Identify hotspots with highest centrality and fragility. |
| `scouter_dependencies` | **Audit**: Map inter-file and package-level dependencies. |

---

## 🔮 Roadmap (v3.0 — Divine Sovereignty)
- **Hybrid Search**: Merging BM25 with Local Vector Embeddings (RRF Fusion).
- **CI/CD Oracle**: Automatic test prioritization based on Blast Radius.
- **Real-time Watching**: Instant index updates via LSP file-watcher integration.

## 📜 License
MIT — *The signal must flow.*
