# Scouter 🕶️

**The high-precision code analysis engine for AI agents.**  
*Surgical AST indexing. Byte-safe snippet reading. SQLite persistence. Zero dependencies.*

Scouter gives AI agents (like Gemini CLI, OpenCode, and Claude Code) "X-ray vision" for your codebase. Instead of reading entire files (wasting tokens and context), Scouter analyzes the AST and provides surgical access to functions, classes, and methods.

## 🚀 Quick Start

### 1. Build from source
Requires Go 1.25+

```bash
git clone https://github.com/Rogercode97/scouter
cd scouter
make build
```

### 2. Automatic Setup
Scouter can self-configure your favorite AI agents:

| Agent | Command |
|-------|---------|
| **Gemini CLI** | `./bin/scouter setup gemini-cli` |
| **OpenCode** | `./bin/scouter setup opencode` |

*Note: Setup for Claude Code and Codex coming soon.*

---

## 🛠️ How it works

Scouter runs as an **MCP (Model Context Protocol)** server. It indexes your project's structure and stores it in a local SQLite database (`~/.scouter/scouter.db`).

1. **Index:** `scouter_index` analyzes a file and returns its symbols (AST pointers).
2. **Cache:** Results are cached based on file `mtime`. Subsequent reads are instant.
3. **Read:** `scouter_read` uses the pointers to extract the exact byte range of a function or class.

## 📦 Distribution
Scouter is a **single 13MB binary**. It embeds its own TypeScript plugins for OpenCode using Go's `//go:embed` directive, making it ultra-portable and easy to install on any machine.

## 📜 License
MIT
