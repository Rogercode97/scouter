# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
