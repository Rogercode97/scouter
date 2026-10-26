# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **Propagate**: `scouter propagate` no longer panics with a nil-pointer dereference. The CLI wires `StructuralTransformer` (`$SYMBOL` pattern) and accepts only the `rename:NewName` form; anything else is rejected with usage. `RippleEngine.Propagate` fails fast with a descriptive error when no Transformer is configured.
- **Ledger**: `StructuralRefactor` no longer stages ast-grep's diff rendering as file content (which `commit` then wrote into sources). It rewrites a scratch copy in place with `--update-all` and stages the copy's content; the original is never touched. `sg`/`ast-grep` allow-list gains the fixed `--update-all` flag.
- **CI**: dropped `-race` (it requires cgo and always failed) and pinned `CGO_ENABLED: "0"`, so CI enforces the zero-CGO mandate instead of contradicting it.

### Added
- **MCP Auto-Sync Watcher**: Scouter MCP now runs a native background file watcher (`fsnotify`) with a 250ms debounce to automatically keep the AST SQLite index in sync with IDE saves, eliminating manual indexing.

### Changed
- **Architecture**: Migrated CLI routing from a monolithic switch (`internal/cli/cli.go`) to `github.com/spf13/cobra` framework (`cmd/scouter/scoutercmd/*`), preserving 100% backward compatibility via proxy fallback.
- **Architecture**: Abstracted infrastructure logic (AST-Grep execution, RTK detection) from the MCP handler (`handle_analysis.go`) into dedicated domain adapters (`internal/adapters/astgrep`, `internal/adapters/rtk`) enforcing Clean Architecture.

## [1.2.1] - 2026-06-14

### Fixed
- **Audit**: Resolved 4R security and performance issues (#36)
- **CLI**: Initialized semanticEngine to fix build and CI pipelines

## [2.7.0] - 2026-05-09

### Added
- **Full MCP Compliance**: Implemented missing MCP Prompts (`self_heal`, `gep_mutator`, `compact_context`, `judge`).
- **Expanded Resources**: Exposed ADRs, Staging Ledger, and Dependency Graph via MCP URIs (`scouter://`).
- **Release Automation**: Integrated GoReleaser and GitHub Actions for automated multi-platform builds.
- **Dynamic Versioning**: Version is now injected at build time.

### Fixed
- MCP SDK v1.6.0 compatibility issues (Role typing and TextContent structure).
- Flaky unit test `TestExecuteEcho` in `internal/engine`.

### Changed
- Refactored MCP server initialization to be more modular.
- Updated CLI versioning strategy to follow the "Gentleman Standard".
