# 🤖 Scouter Agent Instructions (Truth Kernels)

## 📌 CORE IDENTITY
Scouter is a high-performance, CGO-free, single-binary CLI engine for AST analysis, blast radius mapping, Git churn risk assessment, and safe atomic code mutations via Staging Ledger (2PC). It operates strictly as an on-demand CLI, synergizing with CodeGraph (which serves as the fast knowledge graph reader for symbol exploration).

## 🏗️ ARCHITECTURAL MANDATES
- **Zero CGO**: The project runs purely on Go. SQLite relies on `modernc.org/sqlite` with `modernc.org/sqlite/vec`. Never introduce CGO. Ensure `CGO_ENABLED=0` is explicitly set in build/test targets to ensure portable, dependency-free cross-compilation.
- **Pure CLI Architecture (Antifragility)**: Scouter runs on-demand without persistent background daemons or file watchers to protect system memory and inotify handles.
- **SQLite Concurrency & Resource Discipline**:
  - Use connection pools configured with WAL mode (`journal_mode(WAL)`), busy timeouts, and synchronous NORMAL.
  - Single writer pool (`MaxOpenConns(1)`) paired with concurrent reader pool (`runtime.NumCPU() * 2`).
  - Use **Bulk Updates** for DB writes without nested transactions on writer pools.
- **Ledger Mutation Protocol**: Any structural code changes MUST be buffered in memory using the `Ledger` before atomic disk flush. Direct destructive mutation is forbidden. Staged patches MUST populate the `Original` content from disk to prevent `Rollback` from deleting existing source files.
- **Command Security**: Never execute raw commands via `exec.Command`. MUST use `internal/utils/safe_exec.go` (`SafeCommand`) to prevent command injection.
- **Index Ignore Contract**: `index` and structural search respect the repo-root `.gitignore` plus `.git/info/exclude` via the shared `IgnoreMatcher` (`internal/engine/ignore.go`, go-git matcher already in `go.mod`, no new dependency). Nested `.gitignore` files are out of scope. `BatchLoadPackages` still parses ignored files; the filter applies at indexing time (correctness first, parse cost belongs to T5).
- **Vector Orphan Invariant**: `vec_symbols` is `vec0` without `ON DELETE CASCADE`. Every path-clearing transaction MUST call `ClearSemanticVectors` before `ClearSymbols` (both `SaveFileIndexBatch` and delta-sync). Symbol reads used to feed vec inserts MUST select `id` (`GetSymbolsByNameInFile` omits it; resolve the real id explicitly).

## ⚙️ EXECUTION & RDD/TDD RULES
- **Receipt-Driven Development (RDD)**: Live changes are validated through `gentle-ai review assess` and `gentle-ai review start` before committing.
- **Targeted TDD**: Use direct, targeted testing (`CGO_ENABLED=0 go test ./specific_pkg`) to conserve execution time and context.
- **Concurrency Verification Without -race**: `-race` requires cgo and this project is CGO-free, so it never runs here. Concurrency changes are proven with `BenchmarkMassiveIndexing` (no deadlock, direction check) plus a `GOMAXPROCS=1` probe and the focused package suite.
- **Toolchain Pin (`.go-version` = 1.26.3)**: Go 1.27.x has an upstream data race inside `go/types` (`(*Named).unpack` vs `(*Checker).isComplete`) that `packages.Load` triggers when packages are type-checked concurrently. It reproduces on 1.27.1 and 1.27.3 and is clean on 1.26.3; `TestHealerEngine_Cancellation` fails 3 of 3 under `-race` on 1.27.x. A `toolchain` line in `go.mod` does **not** pin anything (it is a floor under `GOTOOLCHAIN=auto`; verified: a module declaring `toolchain go1.26.3` still reported `GOVERSION=go1.27.1`), so CI reads `.go-version` via `go-version-file` and sets `GOTOOLCHAIN=local`. Locally, run any `-race` command with `GOTOOLCHAIN=go1.26.3 go test -race ...` until upstream fixes it.
- **Load Mode Floor**: `BatchLoadPackages` and `loadASTFile` keep full `NeedTypes|NeedTypesInfo` with `Tests:true` — Go recall 1.00 depends on `TypesInfo.Selections/Uses`, and test files hold call edges. `bench-baseline.sh` 12643ms → 12434ms on 12 CPUs proved workers were never the bottleneck; the gate is no-regression, not a speedup.
- **Token Killer**: Leverage `-u` / `--ultra-compact` to maximize context efficiency when querying Scouter outputs.

## 📁 PROJECT MAP
- `cmd/scouter/` - CLI entry and Cobra subcommands (`scoutercmd/`).
- `internal/engine/` - Impact (blast radius), Evolution (Ledger 2PC), Churn (Git hotspots), Rules (AST), Diagnostic.
- `internal/store/` - SQLite persistence for symbols, calls, data flows, and churn metrics.
- `odd/` - Organic Driven Development tasks and execution records.

## 🔄 SESSION BOOTSTRAPPING
- **Engram First**: Proactively consult Engram (`mem_search` or `mem_context`) to retrieve prior architectural decisions and workflows before reading code or mutating files.