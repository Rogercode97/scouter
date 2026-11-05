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

## ⚠️ THREE WAYS TO HURT THIS MACHINE

Each of these has already cost work here. None is hypothetical.

1. **Index over the global database.** `index` writes to `~/.config/scouter/scouter.db` (`internal/config/config.go:47`), the developer's live index. Any measurement, scratch repo or manual probe sets `SCOUTER_CONFIG` to a TOML naming a temporary `[tracking] db_path` (`config.go:112`). Copying data in is fine; writing back out is not.
2. **Killing by pattern.** The LSP manager starts a shared `gopls` daemon on `/tmp/scouter-gopls.sock` (`internal/engine/lsp/manager.go:120`) that other sessions use. Never `pkill -f gopls`, `pkill -f scouter`, or kill a PID matched by name, path or worktree string: this session's own process carries the worktree path in argv. Kill a PID captured at spawn, or the socket owner from `ss -H -lxnp` after confirming `/proc/<pid>/cwd` is your worktree.
3. **Sharing the staging ledger.** `.scouter/staging/ledger.json` lives in the cwd, so two sessions in one worktree stage into the same buffer and whichever commits first writes the other's files. Isolate with a distinct worktree and `SCOUTER_CONFIG` when a task will stage mutations.

## 🎯 SURFACES: FIX ALL OF THEM

The most common defect here is a change that works on the surface it was tested on and is missing everywhere else. Before calling a command change done, state for each row whether it applied:

| Surface | Where it lives | When it needs a decision |
| :--- | :--- | :--- |
| CLI subcommand | `cmd/scouter/scoutercmd/` | Always, if the behavior is reachable from a command |
| MCP tool | `internal/mcp/server.go` (`AddTool`) | The agent-facing equivalent; a capability only reachable through handler code is not reachable at all |
| Agent skill | `.agents/skills/scouter/SKILL.md` | When the workflow an agent runs changes |
| Help text | Cobra `Short` / `Long` / flag usage | Any new or renamed flag, command or message |
| Docs | `README.md`, `docs/` | When the change makes existing guidance inaccurate |
| Reverse state | Any new way in | A way out and a way to see it. Staged needs inspect and rollback; a new flag needs a way to not use it |

## ⚙️ EXECUTION & RDD/TDD RULES
- **Receipt-Driven Development (RDD)**: Live changes are validated through `gentle-ai review assess` and `gentle-ai review start` before committing.
- **Targeted TDD**: Use direct, targeted testing (`CGO_ENABLED=0 go test ./specific_pkg`) to conserve execution time and context.
- **Test what can break; nothing else**: orchestration and persistence, recall/precision of the analysis engines, `IgnoreMatcher` and path validation boundaries, external-format parsing (tree-sitter, `go/packages`), ledger two-phase commit, and the exact logic a fix changed. Write no test for what the types already guarantee, trivial branching, copy or message text, smoke runs, mocks mirroring calls, or fixtures rendered to static output. Same weight for error, validation and edge clauses as for the happy path.
- **Extend before you add**: when an existing test or table covers the behavior, add a case or an assertion there. A new test function needs a reason no existing case fits.
- **One layer, the lowest that fails**: prove a defect at the layer where it actually breaks. Do not repeat the same case in an engine unit test and again in `tests/` integration, and do not re-assert a store invariant through three commands.
- **No wall-clock assertions**: `time.Sleep`, polling loops and `time.Since`/`time.Now` deltas do not belong in tests. Assert on observable effects — rows written, events emitted, exit codes — never on how long something took. A test that needs a timeout or a retry to pass is wrong; fix the flake, do not raise the ceiling. Synchronize with `sync.WaitGroup`, channels or a context that the code under test closes (`TestHealerEngine_Cancellation` exists for this and takes 16–80s under `-race`; prefer the deterministic signal when you touch it).
- **Concurrency Verification**: `-race` requires cgo, so it cannot run in the `CGO_ENABLED=0` configuration that ships. Run it as its own pass, never mixed into the zero-CGO gate: `GOTOOLCHAIN=go1.26.3 CGO_ENABLED=1 go test -race -p 1 ./<pkg>`. For indexer concurrency also keep `BenchmarkMassiveIndexing` (no deadlock, direction check) plus a `GOMAXPROCS=1` probe, which prove progress where the detector only proves absence of a race.
- **Toolchain Pin (`.go-version` = 1.26.3)**: Go 1.27.x has an upstream data race inside `go/types` (`(*Named).unpack` vs `(*Checker).isComplete`) that `packages.Load` triggers when packages are type-checked concurrently. It reproduces on 1.27.1 and 1.27.3 and is clean on 1.26.3; `TestHealerEngine_Cancellation` fails 3 of 3 under `-race` on 1.27.x. A `toolchain` line in `go.mod` does **not** pin anything (it is a floor under `GOTOOLCHAIN=auto`; verified: a module declaring `toolchain go1.26.3` still reported `GOVERSION=go1.27.1`), so CI reads `.go-version` via `go-version-file` and sets `GOTOOLCHAIN=local`. Locally, run any `-race` command with `GOTOOLCHAIN=go1.26.3 go test -race ...` until upstream fixes it.
- **Load Mode Floor**: `BatchLoadPackages` and `loadASTFile` keep full `NeedTypes|NeedTypesInfo` with `Tests:true` — Go recall 1.00 depends on `TypesInfo.Selections/Uses`, and test files hold call edges. `bench-baseline.sh` 12643ms → 12434ms on 12 CPUs proved workers were never the bottleneck; the gate is no-regression, not a speedup.
- **Token Killer**: Leverage `-u` / `--ultra-compact` to maximize context efficiency when querying Scouter outputs.

## 📁 PROJECT MAP
- `cmd/scouter/` - CLI entry and Cobra subcommands (`scoutercmd/`).
- `internal/engine/` - Impact (blast radius), Evolution (Ledger 2PC), Churn (Git hotspots), Rules (AST), Diagnostic.
- `internal/store/` - SQLite persistence for symbols, calls, data flows, and churn metrics.
- `internal/mcp/` - The agent-facing surface (`server.go`) plus its thin handlers; 27 tools.
- `.agents/skills/scouter/` - The skill an agent reads before driving this CLI.
- `odd/` - Organic Driven Development tasks and execution records.

## 🔄 SESSION BOOTSTRAPPING
- **Engram First**: Proactively consult Engram (`mem_search` or `mem_context`) to retrieve prior architectural decisions and workflows before reading code or mutating files.