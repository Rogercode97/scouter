# Proposal: sovereign-context-wrappers

## Intent
Evolving Scouter's density engine from basic MUNCH to telemetry-aware Sovereign Wrappers. This change implements high-fidelity context compression for Milestone #5948, targeting 95-98% token reduction by prioritizing semantic relevance via state-aware filters.

## Scope
- **In Scope**: `SovereignContext` wrappers, Hot/Warm/Cold state machine, ACCP frame management, ULMEN semantic validation, telemetry hooks, and MCP integration.
- **Out of Scope**: Full replacement of `ast-grep` engine, real-time UI visualization.

## Capabilities
- **New**: `sovereign-context-wrappers` (Multi-tier state management).
- **Modified**: `munch-density-format` (State-aware headers).

## Technical Approach
Implementation of `SovereignContext` as a standard `context.Context` wrapper. It will manage transitions between **Hot** (uncompressed), **Warm** (summarized), and **Cold** (pruned) states based on architectural frames (ACCP) and agent attention. ULMEN will be used to ensure semantic integrity isn't compromised during aggressive compression.

## Success Criteria
- 95-98% token reduction in benchmarks.
- 100% ULMEN pass rate.
- Stable integration with 315 existing tests passing.