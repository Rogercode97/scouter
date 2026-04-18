# Scouter 🕶️ (Wave 8.7 (Redemption Edition))
**Status**: Fully Redemed. Context-Aware I/O (Go 1.25) & MCP Integrated.

**The high-precision, semantic code analysis engine for AI agents.**  
*Surgical AST indexing. FTS5 Semantic Search. Multi-language Tree-sitter. SHA-256 Hashing.*

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

1.  **Index (`scouter_index`)**: Analyzes a file and returns its symbols (AST pointers). Supports **Go, TypeScript, JavaScript, and Python** via Tree-sitter.
2.  **Search (`scouter_search`)**: Uses **FTS5 (Full Text Search)** to locate any symbol (function, class, variable) across the indexed workspace in milliseconds.
3.  **Integrity (`SHA-256`)**: Implements content-based caching. Even if a file's `mtime` changes, Scouter uses SHA-256 hashes to verify if re-indexing is actually needed.
4.  **Read (`scouter_read`)**: Uses AST pointers to extract the exact byte range of a symbol, reducing context usage to the absolute minimum.

## 📦 Distribution
Scouter is a **single 13MB binary**. It is portable, zero-dependency, and embeds its own TypeScript plugins for OpenCode using Go's `//go:embed` directive.

## 📜 License
MIT
