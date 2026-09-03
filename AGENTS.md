# 🤖 Scouter Agent Instructions (Truth Kernels)

## 📌 CORE IDENTITY
Scouter is a CGO-free, single-binary AST-based code analysis and intelligence engine exposed via MCP.

## 🏗️ ARCHITECTURAL MANDATES
- **Zero CGO**: The project runs purely on Go. SQLite relies on `ncruces/go-sqlite3`. The Semantic Engine runs entirely in Go using `goformer`. Never introduce CGO. Ensure `CGO_ENABLED=0` is explicitly set in build/test targets to prevent Termux clang linker failures.
- **Antifragility (Termux/WASM)**: SQLite runs in a WASM sandbox (`wasm2go`). To prevent massive overhead, DB locking, and OOM crashes:
  - You MUST configure connection limits and memory contexts (`sqlite3.WithMaxMemory`).
  - You MUST use **Bulk Updates** for DB writes without nesting transactions (nested transactions on a single `MaxOpenConns(1)` writer will deadlock).
  - You MUST use **Circuit Breakers** and **Dual Connection Pools** for SQLite.
- **Decoupled Roles**: The `TruthEngine` God Object is dead. Enforce strict role-based interfaces (`IndexerService`, `HealerService`). Keep the Presentation layer (`display.Presenter`) separate from MCP handlers.
- **Context Integrity**: Always propagate `context.Context` down the stack. 
- **Command Security**: Never execute raw commands via `exec.Command` for dynamically formed shells. MUST use `internal/utils/safe_exec.go` (`SafeCommand`) to prevent command injection via binary allow-listing.
- **Ledger Mutation Protocol**: Any structural code changes MUST be buffered in memory using the `Ledger` before atomic disk flush. Direct destructive mutation is forbidden. Staged patches MUST populate the `Original` content from disk to prevent `Rollback` from permanently deleting existing source files.

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

## 🔄 SESSION BOOTSTRAPPING
- **Engram First**: At the start of any new session or task in this project, you MUST proactively consult Engram (`mem_search` or `mem_context`) to retrieve prior architectural decisions, workflows, and context before reading code or taking action.