# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Sections are ordered newest first. `2.7.0` predates the renumbering to `1.0.0`
and is kept at the bottom for history.

## [Unreleased]

### Fixed
- **Indexer**: a Python call written through an import alias resolves to the symbol it names, and keeps the module with the name. `import core as c` followed by `c.boost(i)` used to record a callee called `c.boost`, which no symbol answers to, so the caller was invisible to `impact`; the same held for `from helpers import normalize as norm` followed by `norm(i)`. Callees are now mapped through the file's import table and qualified by module, so a call through an import never lands on a same-named definition elsewhere. When one file binds the same alias to two modules the call is left unresolved rather than guessed. Measured recall on the impact corpus went from 0.40 to 0.60.
- **Impact**: a short symbol name that names two different definitions is refused instead of answered. The resolver keyed candidates on the rendered fully-qualified name, so in a language that attaches no package path to a symbol both definitions collapsed into one candidate and the blast radius merged two call graphs. The identity is now the definition (name and file), and the error names both files.

## [1.4.0] - 2026-11-03

### Added
- **Doctor**: new report-only `scouter doctor` checks ast-grep, database and ledger health before indexing or mutating, with the exact remedy per failure.
- **Demo**: `just demo` reproduces the 30-second portfolio demo (`docs/demo.md`, replayable `assets/demo.cast`) without touching state.
- **Impact**: `impact` and `graph` resolve short symbol names to FQN; a name that matches nothing is an error, never a silent empty blast radius, and a name matching several symbols is refused rather than guessed. For symbols that carry no package path the ambiguity was still collapsed until the fix recorded under Unreleased.
- **Propagate**: skips byte-identical transforms (no turn, budget or mtime churn) and warns when the old name survives in rewritten files (qualified call sites).
- **Context**: single-shot, token-dense context packet for agent handoff.
- **Search**: results ranked by `runtime_hits` frecency, with a best-effort bump.
- **Store**: SQLite errors are annotated with the failing operation and result code instead of surfacing a bare driver message.
- **Skills**: workspace project skill describing the Scouter CLI surface.

### Changed
- **Store**: migrated from the ncruces WASM SQLite driver to `modernc.org/sqlite` (pure Go), with POSIX `flock` protection for the concurrent pools.
- **Indexer**: heavy semantic-embedding loading is gated behind `--deep`, so a plain index run stays cheap.
- **Engine**: the git engine is unified on `go-git/v5` and the v6 alpha is dropped. `HealerEngine` takes an injected diagnostic provider, an `Embedder` interface is extracted from `SemanticEngine`, and the MCP presentation layer is decoupled from the server.
- **CI**: `CGO_ENABLED=0` enforced for local and Termux builds, `gosec` pinned instead of floating on `master`, the stray nested module under `internal/` removed, and PR type labels auto-assigned from branch name or title.

### Fixed
- **Propagate**: `scouter propagate` no longer panics with a nil-pointer dereference. The CLI wires `StructuralTransformer` (`$SYMBOL` pattern) and accepts only the `rename:NewName` form; anything else is rejected with usage. `RippleEngine.Propagate` fails fast with a descriptive error when no Transformer is configured.
- **Ledger**: `StructuralRefactor` no longer stages ast-grep's diff rendering as file content (which `commit` then wrote into sources). It rewrites a scratch copy in place with `--update-all` and stages the copy's content through an open handle, never by reopening the path; the original is never touched.
- **Ledger**: one canonical ledger. `EvolutionEngine` mirrors staged patches into its own staging path instead of letting two `Ledger` objects diverge in memory. Each commit is recorded in the journal and an interrupted commit is resolvable; the crash-safety guarantee is now stated where the code implements it.
- **Apply**: a failure arriving together with an interruption is no longer swallowed, and the apply mode is durable. Backed by a deterministic interruption seam and a durability barrier so the case is testable.
- **Engine**: `rollback` could delete existing source files; staged patches now populate `Original` from disk and commit orchestration is unified. An `ast-grep` binary is resolved instead of assuming `sg` is on `PATH`.
- **Security**: `SafeCommand` rejects shells and `eval`, and rejects flags that are not allow-listed for the binary.
- **Filter**: failure paths containing a hyphen or a space are parsed correctly.
- **Search**: a path prefix that cannot match is rejected, and `.` is no longer passed through as a prefix.
- **Store**: existing databases converge to the current schema instead of requiring a fresh index.
- **CI**: the unit job pins `CGO_ENABLED=0` and no longer passes `-race`, which requires cgo; race detection stays in the dedicated `Race Detector` job (`go test -race`, default cgo). The structural-search cancellation test no longer depends on a 500-file search being slower than 10 ms.
- **Bench**: a failing `ripgrep` aborts the run instead of recording a zero-byte baseline that reads as a real measurement.
- **Deps**: `go-git/v5` 5.19.2 for symlink traversal and malicious reference names.

### Removed
- **MCP auto-sync watcher**: the background `fsnotify` watcher that kept the index current was pruned along with the dependency. The pure-CLI mandate forbids persistent daemons and file watchers, so the index only advances when a client asks for it, through `scouter index` or the MCP `ast_index` tool.

## [1.3.0] - 2026-06-23

### Added
- **Tests**: table-driven coverage for `EvolutionEngine`, replacing ad-hoc test scaffolding.

### Changed
- **Architecture**: CLI routing migrated from a monolithic switch (`internal/cli/cli.go`) to the `spf13/cobra` framework (`cmd/scouter/scoutercmd/*`), preserving the previous behaviour through a proxy fallback.
- **Architecture**: AST-Grep execution and RTK detection extracted from the MCP handlers (`handle_analysis.go`) into dedicated adapters (`internal/adapters/astgrep`, `internal/adapters/rtk`).
- **Architecture**: `DiagnosticEngine` decoupled from `HealerEngine` through a Solver-Verifier pipeline, breaking the cyclic dependency; `EvolutionEngine` and `ChronosEngine` extracted as their own engines.

### Fixed
- Nil-pointer panic in the `service_test.go` mock.
- `SemanticDiff` signature mismatch in tests.
- Race condition in `handleDream`.
- `SearchSemantic` now enforces `SafeCommand`, and the churn engine's iteration hack is gone.

### Chore
- Dependency bumps: `golang.org/x/mod` 0.37.0, `golang.org/x/tools` 0.46.0, `actions/checkout` 7.
- Purged accidentally committed binaries and scripts; ADR and architectural maps updated for the Cobra migration.

## [1.2.1] - 2026-06-14

### Fixed
- **Audit**: resolved 4R security and performance issues (#36).
- **CLI**: initialized `semanticEngine` to fix the build and CI pipelines.

### Chore
- Dependency bumps: `golang.org/x/sync` 0.21.0, `gotreesitter` (#31, #32).

## [1.2.0] - 2026-06-05

### Added
- **Hybrid search**: FTS5 and `sqlite-vec` reciprocal-rank fusion computed inside SQL.
- **MCP tools**: `risk_semantic_diff` for AST structure diffing and `risk_lint_architecture` for AST governance.
- **Setup**: automated MCP setup commands for seven clients, documented in the README.

### Changed
- Bleve extirpated in favour of native SQLite FTS5.
- `TruthEngine` migrated to the CLI and the SDD workflow purged from it.

### Fixed
- Memory leaks via explicit tree release and a parser `sync.Pool`.
- CGO injection RCE and unsafe filepath handling; CGO mitigations applied for real.

## [1.1.0] - 2026-06-04

### Added
- **Graph analytics**: PageRank, centrality and interface analysis pushed into SQLite temp tables with bulk updates, crossed through the WASM boundary as few times as possible.

### Changed
- `TruthEngine` decoupled, `Presenter` extracted, and SQLite split into dual connection pools (isolated per test).

### Fixed
- gosec false positive on hardcoded credentials.

## [1.0.0] - 2026-06-03

First tagged release: the AST indexing, impact, churn and staging-ledger CLI as a
single CGO-free binary.

## [2.7.0] - 2026-05-09

Released before the renumbering to `1.0.0`.

### Added
- **Full MCP Compliance**: implemented the missing MCP Prompts (`self_heal`, `gep_mutator`, `compact_context`, `judge`).
- **Expanded Resources**: exposed ADRs, the Staging Ledger and the dependency graph through `scouter://` URIs.
- **Release Automation**: GoReleaser and GitHub Actions for multi-platform builds.
- **Dynamic Versioning**: the version is injected at build time.

### Fixed
- MCP SDK v1.6.0 compatibility (role typing and `TextContent` structure).
- Flaky `TestExecuteEcho` in `internal/engine`.

### Changed
- MCP server initialization refactored to be modular.
- CLI versioning strategy realigned.
