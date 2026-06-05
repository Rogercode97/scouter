# 🤖 Scouter Agent Instructions (Truth Kernels)

## 📌 CORE IDENTITY
Scouter is a CGO-free, single-binary AST-based code analysis and intelligence engine exposed via MCP.

## 🏗️ ARCHITECTURAL MANDATES
- **Zero CGO**: The project runs purely on Go. SQLite relies on `ncruces/go-sqlite3`. The Semantic Engine runs entirely in Go using `goformer`. Never introduce CGO.
- **Antifragility (Termux/WASM)**: Wazero interpreter lacks JIT. To prevent massive Wasm-Host FFI overhead and DB locking:
  - You MUST use **Bulk Updates** for DB writes.
  - You MUST use **Circuit Breakers** and **Dual Connection Pools** for SQLite.
- **Decoupled Roles**: The `TruthEngine` God Object is dead. Enforce strict role-based interfaces (`IndexerService`, `HealerService`). Keep the Presentation layer (`display.Presenter`) separate from MCP handlers.
- **Context Integrity**: Always propagate `context.Context` down the stack. 

## ⚙️ EXECUTION & TDD RULES
- **Development TDD**: Use direct, targeted testing (`go test ./specific_pkg`) to conserve context.
- **Pre-Push Gate**: You MUST run `just test` (full test suite execution) before committing or pushing changes.

## 📁 PROJECT MAP
- `cmd/scouter/` - CLI entry & Server logic.
- `internal/mcp/` - Protocol Transport (Handlers). No presentation logic allowed.
- `internal/engine/` - Search, Impact, and Refactoring engines.
- `internal/store/` - Persistent indexing.
- `openspec/` - Specs and Truth tracking.

## 🧠 MCP RESPONSES
- Always use `<thought>...</thought>` blocks for complex analysis or multi-step logic before returning tool results.