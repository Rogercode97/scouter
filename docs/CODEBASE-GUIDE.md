# 🗺️ Scouter Codebase Guide

This guide provides a structural map of the Scouter repository to help you navigate and understand the flow of data and responsibility.

## 📂 Directory Map

| Directory | Responsibility | Key Symbols |
| :--- | :--- | :--- |
| `cmd/scouter` | Entry point for the CLI and MCP Server. | `main.go` |
| `internal/mcp` | MCP Server implementation and Tool adapters. | `Server`, `handleSearch` |
| `internal/engine` | Core analytical and mutation logic (TruthEngine). | `TruthEngine`, `RippleEngine` |
| `internal/store` | Persistence layer (SQLite, FTS5, Call Graph). | `Store`, `migrate` |
| `internal/adapters`| External integrations (Engram, LLM Sampling). | `MemoryProvider` |
| `internal/display` | UI logic for CLI output and formatting. | `Display`, `Gain` |
| `openspec` | SDD (Spec-Driven Development) artifacts. | `specs.md`, `tasks.md` |

## 🚀 Critical Paths

### 1. The Boot Sequence
Located in `cmd/scouter/main.go`. It initializes the `Store`, injects it into the `TruthEngine`, and launches either the CLI or the MCP Server.

### 2. Analytical Request Flow
1. **MCP Handler** (`internal/mcp/handlers.go`): Receives the JSON-RPC request.
2. **TruthEngine** (`internal/engine/truth.go`): Orchestrates the request (e.g., `AnalyzeImpact`).
3. **Specialized Engine**: The request is delegated (e.g., `internal/engine/impact.go`).
4. **Store**: Fetches the required AST symbols or Call Graph data.

### 3. Evolutionary Change Flow
1. **Ripple/Evolve**: A mutation is proposed.
2. **Impact Analysis**: The "blast radius" is calculated and verified.
3. **Staging Ledger**: The change is staged in memory (not yet on disk).
4. **Commit**: The `Ledger` applies the final changes to the filesystem.

## 🛠️ Development Mandates

- **Don't touch the Domain**: Business logic belongs in `internal/engine`. Keep `internal/mcp` as thin as possible.
- **TDD First**: Every new analytical capability MUST have a corresponding test in `internal/engine/*_test.go`.
- **Pure Signal**: Truncate massive outputs. Use the `Display` package for formatting.

---
*The map is not the territory, but a good map prevents a lot of backtracking. Hakai.*
