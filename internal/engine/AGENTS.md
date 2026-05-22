# internal/engine — Agent Instructions

## Overview
The `internal/engine` package contains the core analysis logic for Scouter. It implements the primary analysis engine, coordinating specialized sub-engines to analyze, diagnose, and refactor the codebase.

## Sub-Engines
- `truth.go`: Central orchestrator and dependency injector for the analysis suite.
- `impact.go`: Calculates the blast radius and risk scores of proposed changes.
- `analyzer.go`: Handles AST-based symbol extraction and centrality metrics.
- `healer.go`: Provides automated root cause analysis (RCA) and repair verification.
- `ripple.go`: Manages coordinated change propagation across multiple files.
- `compaction.go`: Optimizes context windows through summarization and filtering.

## Development Guidelines
- **Stateless Design**: Engines should remain stateless where possible, utilizing the storage layer for persistent data.
- **AST Integrity**: Utilize Tree-sitter for all structural analysis to ensure accuracy across different languages. Use `StreamWithTreeSitter` and `iter.Seq` to minimize memory overhead when parsing large files.
- **Rust Support**: Rust trait implementations (`impl Trait for Type`) are explicitly extracted as `implements` links directly by the parser, mapping to the caller/callee relation structure.
- **Verification Mandate**: In `healer.go`, do not apply automated fixes without a corresponding reproduction test.
- **Metric Normalization**: Follow the standardized formulas for calculating impact and risk as defined in the engine logic.

## Operational Boundaries
- ✅ **Required:** Add comprehensive unit tests for all new analysis capabilities.
- ⚠️ **Review Needed:** Consult on modifications to the main analysis engine interface or the introduction of global state.
- 🚫 **Prohibited:** Executing file system modifications without a robust rollback or staging mechanism.
