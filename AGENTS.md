# 🤖 Scouter Agent Instructions (Truth Kernels)

## 📌 CORE IDENTITY
Scouter is a high-performance, CGO-free, single-binary CLI engine for AST analysis, blast radius mapping, Git churn risk assessment, and safe atomic code mutations via Staging Ledger (2PC). It operates strictly as an on-demand CLI in Termux, synergizing with CodeGraph (which serves as the fast knowledge graph reader for symbol exploration).

## 🏗️ ARCHITECTURAL MANDATES
- **Zero CGO**: The project runs purely on Go. SQLite relies on `ncruces/go-sqlite3`. Never introduce CGO. Ensure `CGO_ENABLED=0` is explicitly set in build/test targets to prevent Termux clang linker failures.
- **Pure CLI Architecture (Termux Antifragility)**: Scouter runs on-demand without persistent background daemons or file watchers (`fsnotify` is permanently deprecated to protect mobile memory and inotify handles).
- **WASM SQLite Discipline**: SQLite runs in a WASM sandbox (`wasm2go`). To prevent DB locking, memory spikes, and OOM crashes:
  - Configure connection limits and memory contexts (`sqlite3.WithMaxMemory`).
  - Use **Bulk Updates** for DB writes without nested transactions (nested transactions on a single `MaxOpenConns(1)` writer will deadlock).
  - Use circuit breakers and dual connection pools.
- **Ledger Mutation Protocol**: Any structural code changes MUST be buffered in memory using the `Ledger` before atomic disk flush. Direct destructive mutation is forbidden. Staged patches MUST populate the `Original` content from disk to prevent `Rollback` from deleting existing source files.
- **Command Security**: Never execute raw commands via `exec.Command`. MUST use `internal/utils/safe_exec.go` (`SafeCommand`) to prevent command injection.

## ⚙️ EXECUTION & RDD/TDD RULES
- **Receipt-Driven Development (RDD)**: Live changes are validated through `gentle-ai review assess` and `gentle-ai review start` before committing.
- **Targeted TDD**: Use direct, targeted testing (`CGO_ENABLED=0 go test ./specific_pkg`) to conserve execution time and context.
- **Token Killer**: Leverage `-u` / `--ultra-compact` to maximize context efficiency when querying Scouter outputs.

## 📁 PROJECT MAP
- `cmd/scouter/` - CLI entry and Cobra subcommands (`scoutercmd/`).
- `internal/engine/` - Impact (blast radius), Evolution (Ledger 2PC), Churn (Git hotspots), Rules (AST), Diagnostic.
- `internal/store/` - SQLite persistence for symbols, calls, data flows, and churn metrics.
- `odd/` - Organic Driven Development tasks and execution records.

## 🔄 SESSION BOOTSTRAPPING
- **Engram First**: Proactively consult Engram (`mem_search` or `mem_context`) to retrieve prior architectural decisions and workflows before reading code or mutating files.