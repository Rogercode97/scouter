# internal/engine — Agent Instructions

## Overview
The Engine is the core intelligence of Scouter. It implements the "TruthEngine" pattern, coordinating specialized sub-engines to analyze, heal, and evolve the codebase.

## Sub-Engines
- `truth.go`: Central orchestrator and dependency injector.
- `impact.go`: Calculates blast radius and risk scores (Wave 9 formula).
- `analyzer.go`: AST-based symbol extraction and centrality calculation.
- `healer.go`: Autonomous Root Cause Analysis (RCA) and fixing.
- `ripple.go`: Strategic change propagation using BFS and validation pipelines.
- `compaction.go`: Context window optimization and summarization.

## Development Guidelines
- **Statelessness**: Engines should ideally be stateless, relying on `store.Repository` for data.
- **AST Sovereignty**: Use `go-sitter` for all structural analysis. Avoid regex for code parsing.
- **Falsifiable Hypotheses**: In `healer.go`, never apply a fix without a reproduction test case.
- **Risk Calculation**: Follow the logarithmic normalization formula defined in `impact.go`.

## Boundaries
- ✅ **Always do:** Add unit tests for every new engine capability.
- ⚠️ **Ask first:** Modifying the `TruthEngine` interface or adding global state.
- 🚫 **Never do:** Perform side effects (file writes) without a rollback mechanism.
