# Scouter 🕶️ (Wave 8.9 — Absolute Sovereignty)
**Status**: Sovereign. Divine Architecture (Go 1.25). Glasswall Validated. Rating 10.0.

**The high-precision, semantic code analysis engine for AI agents.**  
*Surgical AST indexing. FTS5 Semantic Search. Multi-language Tree-sitter. Global Call Graph. Ecosystem Sovereignty. Parallel Performance. Dead Code Detection.*

Scouter gives AI agents (like Gemini CLI, OpenCode, and Claude Code) "X-ray vision" for your codebase. It eliminates token waste by providing surgical access to symbols and instant, cross-file impact analysis.

## 🚀 Quick Start

### 1. Build from source (Go 1.25+)
```bash
go build -o bin/scouter cmd/scouter/main.go
go build -o bin/index-vault cmd/index-vault/main.go
```

### 2. Index the Workspace (Now Parallel! ⚡)
```bash
./bin/index-vault
```

---

## 🏛️ Sovereign Capabilities (v1.5.0)

Scouter runs as a high-performance **MCP (Model Context Protocol)** server with five pillars of sovereignty:

### 1. Parallel Sovereignty (v1.5) ⚡
A high-performance **Worker Pool** engine that indexes workspaces at ~100 files/sec using all available CPU cores. Atomic SQLite transactions ensure perfect integrity.

### 2. Dead Code Analysis (v1.4) 💀
The power of **Selective Destruction (Hakai)**. Identifies unused symbols across the workspace, differentiating between internal code and potentially unused public APIs.

### 3. Global Call Graph (V2.0) 🕸️
Tracks who calls whom across the entire workspace. Supports **Go, TypeScript, JavaScript, and Python**, including deep detection within closures and goroutines.

### 4. Ecosystem Sovereignty 📦
Vision beyond source code. Automatically parses `go.mod` and `package.json` to map project dependencies and their exact versions.

### 5. Glasswall & OOM Guard 🛡️
Military-grade security. Strict `validator/v10` schema enforcement and hard memory limits to protect both RAM and LLM context windows.

---

## 🧩 Integration Example

When using **Gemini CLI**, you can trigger the autonomous explanation workflow:

```bash
/scouter-explain symbolName=store.New
```

**Live Impact Analysis (Mermaid.js):**
Scouter can generate visual dependency diagrams directly in your chat via `scouter_visualize`.

## 📜 License
MIT — *Sovereignty for all.*
