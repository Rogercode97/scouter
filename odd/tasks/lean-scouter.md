# Tasks: Lean Scouter v2 (ODD + RDD)

## Invariants & Guardrails
- **Zero CGO**: Enforce `CGO_ENABLED=0` in all build/test commands to avoid Termux clang linker failures.
- **WASM SQLite Antifragility**: SQLite runs on `ncruces/go-sqlite3` in WASM sandbox. No nested transactions on single writer connection.
- **Bounded Batches**: Each Work Unit must not exceed ~400 changed lines.
- **RDD Verification**: Run `gentle-ai review assess` and `gentle-ai review start` before each work unit commit.

## Work Units

- [x] **WU-1: Poda de Daemon MCP y Watcher**
  - Scope: Remove `internal/engine/watcher.go`, remove `fsnotify` dependency, retire/deprecate MCP background daemon from `cmd/scouter/scoutercmd/mcp.go`.
  - Verification: `CGO_ENABLED=0 go test ./...` passes without inotify leaks or background watcher threads.
  - Review: `gentle-ai review assess` -> `gentle-ai review start` -> Commit (Approved, Lineage `review-7ac6947718a62214`).

- [x] **WU-2: Unificación de Git Engine**
  - Scope: Migrate `internal/engine/churn.go` from `go-git/v6` to `go-git/v5`. Prune `v6` from `go.mod`.
  - Verification: `CGO_ENABLED=0 go test ./internal/engine/...` passes.
  - Review: `gentle-ai review assess` -> `gentle-ai review start` -> Commit.

- [ ] **WU-3: Indexer Ligero (Sin Embeddings Obligatorios)**
  - Scope: Decouple mandatory `goformer` inference from `internal/engine/indexer.go`. Ensure `scouter index` runs in <3s with <50MB RAM.
  - Verification: Benchmark / test `scouter index` locally.
  - Review: `gentle-ai review assess` -> `gentle-ai review start` -> Commit.

- [ ] **WU-4: Binario en $PREFIX/bin y Skill CLI**
  - Scope: Compile and install binary to `/data/data/com.termux/files/usr/bin/scouter`. Update `~/.gemini/config/skills/scouter/SKILL.md` and `AGENTS.md`.
  - Verification: `scouter --version` and CLI execution tests.
  - Review: `gentle-ai review assess` -> `gentle-ai review start` -> Commit.
