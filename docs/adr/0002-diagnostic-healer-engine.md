# ADR 0002: Diagnostic Healer Engine

## Status
Accepted

## Context
The initial automated fix implementation in Scouter was limited in its understanding of architectural impact. It would attempt to fix a test failure by sending the error log and the immediate failing line to the LLM. This often resulted in fixes that introduced technical debt, such as circular dependencies or high coupling, because the LLM didn't understand the broader context of the symbol it was modifying.

## Decision
We have extracted the diagnostic and healing logic into a specialized `HealerEngine` (internal/engine/healer.go) that implements a structured diagnostic pattern:

1.  **Deep Root Cause Analysis (RCA)**: Instead of just the top frame, the engine parses the entire stack trace and uses the LSP Manager to resolve symbols and documentation for all involved frames.
2.  **Pre-Fix Impact Analysis**: Before requesting a fix, the engine calculates the Risk Score and Centrality of the target symbol. This data, along with a list of direct callers (blast radius), is provided to the LLM.
3.  **Post-Fix Integrity Check**: After a successful test run, the engine re-indexes the modified file and re-calculates centrality. If the centrality increases significantly, the fix is flagged, alerting the user to potential architectural regression.
4.  **Atomic Operations**: The engine uses a backup/restore mechanism to ensure the codebase remains in a valid state if a fix fails or tests do not pass.

## Consequences
- **Positive**: Higher fidelity fixes that respect architectural boundaries.
- **Positive**: Automated detection of fixes that increase coupling.
- **Positive**: Better documentation and context provided for automated fixing.
- **Negative**: Increased complexity in the `internal/engine` package.
- **Neutral**: Higher dependency on a working LSP server for optimal performance.
