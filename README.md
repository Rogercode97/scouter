# Scouter 🕶️ (Wave 8.9 — Truth & Oracle Edition)
**Status**: Sovereign. Divine Architecture (Go 1.25). Glasswall Validated. Rating 10.0.

**The Sovereign Truth Kernel (RTK) & Oracle Engine for AI Agents.**  
*Semantic Omniscience. SNR Filtering. Tee CAS. Predictive Testing. Risk Mapping.*

Scouter is more than a tool; it is a **Truth Kernel (RTK)** that sits between the OS and your AI agent. It purges noise, amplifies signal, and provides real-time semantic intelligence through an integrated LSP Bridge.

## 🚀 The Truth Kernel Experience

### 1. Build & Activate (Go 1.25+)
```bash
go build -o scouter cmd/scouter/main.go
./scouter init  # Installs the Truth Kernel shell hooks
```

### 2. High-Fidelity Signal (SNR)
Scouter automatically intercepts noisy commands (git, go, npm, etc.) and delivers pure signal to the agent:
```bash
# Collapses 1000 lines of duplicates and noise into a pure signal
scouter cat noisy_logs.txt
```

---

## 🏛️ Sovereign Pillars (v2.6.1)

### 1. Truth Kernel (RTK Absorption) 🧬
**Eliminate Token Waste.** Intercepts command streams using **SNR (Signal-to-Noise Ratio)** filtering.
- **snr_dedup**: Collapses consecutive duplicate lines into single [xN] counters.
- **head_tail**: Smart truncation that preserves critical error contexts while cutting middle noise.
- **Gain Control**: Tiered output verbosity (Compact, Signal, Raw) for budget-conscious agents.

### 2. Tee CAS (Content-Addressable Storage) 📦
**The Memory of the Machine.** Stores every unique execution trace using SHA-256 command hashing. No more redundant logs or re-running failed tests just to "see what happened." The agent can query the CAS for previous truths instantly.

### 3. LSP Bridge (Omniscience v2) 🔮
**Real-time Semantic Intelligence.** Connects Scouter to Language Servers (gopls, tsserver, pyright). 
- **Definition & Hover**: High-precision jump-to-source and type resolution across package boundaries.
- **Cross-Language**: Unified semantic visibility for Go, TypeScript, and Python.

### 4. Oracle Engine (Predictive Testing) 🎯
**Predict the Blast Radius.** Analyzes local changes (`git diff`) and suggests exactly which tests to run. Uses **Recursive CTEs** in SQLite to trace dependencies through infinite levels.

### 5. Glasswall Validation 🛡️
**Military-grade Integrity.** Every tool call is validated against strict schemas. All file reads are verified via SHA-256 pre-flight checks to prevent "stale-read" hallucinations.

---

## 🛠️ MCP & CLI Toolset

| Tool | Capability |
| :--- | :--- |
| `scouter_goto_definition` | **Omniscience**: Jump to real source definition via LSP. |
| `scouter_type_info` | **Omniscience**: Resolve precise types and documentation. |
| `scouter_predict` | **Oracle**: Identify affected tests from local Git changes. |
| `scouter_impact` | **Blast Radius**: Recursive dependency analysis. |
| `scouter_critical_code` | **Risk Map**: Identify hotspots (Centrality & Fragility). |
| `scouter_index` | Deep AST indexing with LSP enrichment. |
| `scouter_search` | BM25 semantic search across symbols and docs. |
| `scouter_visualize` | Generate risk-colored **Mermaid.js** call graphs. |
| `scouter_read` | Surgical fragment reading with SHA-256 validation. |

---

## 🔮 Roadmap (v3.0 — Divine Sovereignty)
- **Hybrid Search**: Merging BM25 with Local Vector Embeddings (SQLite-VSS).
- **Agent Sampling**: Enabling Scouter to request reasoning completions from the AI during tool execution.
- **Real-time Watching**: Instant index updates via LSP file-watcher integration.

## 📜 License
MIT — *The signal must flow.*
