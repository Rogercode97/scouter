# Scouter 🕶️ (Wave 8.2 — Sovereign Edition)
**Status**: Sovereign. Divine Architecture (Go 1.25). Glasswall Validated.

**The high-precision, semantic code analysis engine for AI agents.**  
*Surgical AST indexing. FTS5 Semantic Search. Multi-language Tree-sitter. SHA-256 Hashing. Glasswall Security.*

Scouter gives AI agents (like Gemini CLI, OpenCode, and Claude Code) "X-ray vision" for your codebase. It eliminates token waste by providing surgical access to symbols (functions, classes, methods) and instant semantic discovery.

## 🚀 Quick Start

### 1. Build from source
Requires Go 1.25+

```bash
git clone https://github.com/Rogercode97/scouter
cd scouter
go build -o bin/scouter cmd/scouter/main.go
```

### 2. Automatic Setup
Scouter can self-configure your favorite AI agents:

| Agent | Command |
|-------|---------|
| **Gemini CLI** | `./bin/scouter setup gemini-cli` |
| **OpenCode** | `./bin/scouter setup opencode` |

---

## 🛠️ How it works

Scouter runs as an **MCP (Model Context Protocol)** server. It indexes your project's structure into a local SQLite database (`~/.scouter/scouter.db`) using the **Engram Pattern** for maximum performance and integrity.

1.  **Index (`scouter_index`)**: Analyzes a file and returns its symbols (AST pointers). Supports **Go, TypeScript (tsx), JavaScript (jsx), and Python** via Tree-sitter.
2.  **Search (`scouter_search`)**: Uses **FTS5 (Full Text Search)** to locate any symbol (function, class, variable) across the indexed workspace in milliseconds.
3.  **Integrity (`SHA-256`)**: Implements content-based caching and **Symbol-level Hashing**. Each pointer contains a unique SHA-256 signature for structural verification.
4.  **Read (`scouter_read`)**: Uses AST pointers to extract the exact byte range of a symbol (**Fragment Reading**), reducing context usage to the absolute minimum.

## ⚖️ Sovereign Mandates (Wave 8.2)

- **Glasswall Validation**: All MCP inputs are strictly validated with `validator/v10` to prevent memory and range errors.
- **OOM Guard**: Hard limits on search results (100) and indexing responses (500 symbols) to protect LLM context windows and server memory.
- **Context Authority**: 100% `context.Context` propagation across all layers for safe cancellations and concurrency control.
- **Pure Branding**: Zero legacy references. Unified nomenclature: **Fragments** (not snippets).

## 🧩 Agent Integration

### Gemini CLI
- **Server Instructions**: Automatically guides the LLM to prefer AST search over generic grep.
- **Slash Commands**: Use `/scouter-explain symbolName=X` to trigger an autonomous explanation workflow.
- **Live Status**: Reference `@scouter://status` to see indexing stats in real-time.

## 📦 Distribution
Scouter is a **single 15MB binary**. It is portable, zero-dependency, and embeds its own TypeScript plugins for OpenCode using Go's `//go:embed` directive.

## 📜 License
MIT
