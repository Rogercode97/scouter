# Scouter 🕶️ (Wave 8.9 — Absolute Sovereignty)
**Status**: Sovereign. Divine Architecture (Go 1.25). Glasswall Validated. Rating 10.0.

**The high-precision, semantic code analysis engine for AI agents.**  
*Surgical AST indexing. FTS5 Semantic Search. Global Call Graph. Ecosystem Sovereignty. Test Sovereignty. Ghost Speed.*

Scouter gives AI agents (like Gemini CLI, OpenCode, and Claude Code) "X-ray vision" for your codebase. It eliminates token waste by providing surgical access to symbols, cross-file impact analysis, and real-time health diagnostics.

## 🚀 Quick Start

### 1. Build from source (Go 1.25+)
```bash
go build -o bin/scouter cmd/scouter/main.go
go build -o bin/index-vault cmd/index-vault/main.go
```

### 2. Index the Workspace (Ghost Speed ⚡)
```bash
./bin/index-vault
```

### 3. Record Project Health
```bash
go test -json ./... | ./bin/index-vault -health
```

---

## 🏛️ Sovereign Pillars (v1.9.0)

Scouter is built on eight pillars of technical sovereignty:

### 1. Test Sovereignty (v1.9) 🧪
The **Code Physician**. Indexes test results and maps failures directly to their source symbols. Agents can diagnose regressions without reading massive logs.

### 2. Semantic Search (v1.8) 🔍
Intelligent lookups using **BM25 ranking** across names and documentation. Find logic based on concepts (e.g., "db connection") rather than just literal names.

### 3. Ghost Speed (v1.7) ⚡
Incremental indexing that skips unchanged files using SHA-256 pre-flight hashing. Performance reaches **+750 files/sec**.

### 4. Global Call Graph (v2.0 Core) 🕸️
Full visibility into "who calls whom", including **Closure Sovereignty** for tracking calls inside goroutines and anonymous functions.

### 5. Documentation Sovereignty 📚
Indexes GoDoc, JSDoc, and Python Docstrings. Provides 'Intent Visibility'—understanding *why* code exists with minimal context usage.

### 6. Ecosystem Sovereignty 📦
Vision beyond source code. Automatically maps project dependencies and versions from `go.mod` and `package.json`.

### 7. Dead Code Analysis (Hakai) 💀
Identifies orphan symbols across the workspace to automate technical debt elimination.

### 8. Glasswall Security 🛡️
Military-grade input validation and **OOM Guards** on all MCP endpoints.

---

## 🛠️ MCP Toolset

| Tool | Capability |
| :--- | :--- |
| `scouter_index` | Analyze and index file AST, calls, and docs. |
| `scouter_search` | BM25 semantic search across symbols and documentation. |
| `scouter_callers` | List all project-wide invocations of a symbol. |
| `scouter_visualize` | Generate a **Mermaid.js** diagram of dependencies. |
| `scouter_health` | Diagnose failed tests and linked symbols. |
| `scouter_dependencies` | List indexed Go/NPM libraries and versions. |
| `scouter_dead_code` | Audit the project for orphan symbols. |
| `scouter_read` | Surgical fragment reading with SHA-256 validation. |

## 📜 License
MIT — *Sovereignty for all.*
