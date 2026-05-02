# ADR 0002: Sovereign Healer Engine

## Status
Accepted

## Context
The initial self-healing implementation in Scouter was "blind" to architectural impact. It would attempt to fix a test failure by sending the error log and the immediate failing line to the LLM. This often resulted in "successful" test runs that introduced significant technical debt, such as circular dependencies or high coupling, because the LLM didn't understand the broader context of the symbol it was modifying.

## Decision
We have extracted the self-healing logic into a specialized `HealerEngine` (internal/engine/healer.go) that implements the "Sovereign Healer" pattern:

1.  **Deep RCA**: Instead of just the top frame, the engine parses the entire stack trace and uses the LSP Manager to resolve symbols and documentation for all involved frames.
2.  **Pre-Fix Impact Analysis**: Before requesting a fix, the engine calculates the Risk Score and Centrality of the target symbol. This data, along with a list of direct callers (blast radius), is injected into the LLM prompt.
3.  **Post-Fix Integrity Check**: After a successful test run, the engine re-indexes the modified file and re-calculates centrality. If the centrality increases by more than 20%, the fix is flagged with a `SUCCESS_WITH_WARNING` status, alerting the user to potential architectural regression.
4.  **Atomic Operations**: The engine uses a backup/restore mechanism to ensure the codebase remains in a valid state if a fix fails or tests do not pass.

## Consequences
- **Positive**: Higher fidelity fixes that respect architectural boundaries.
- **Positive**: Automated detection of "dirty fixes" that increase coupling.
- **Positive**: Better documentation and context provided to the sampling agent.
- **Negative**: Increased complexity in the `internal/engine` package.
- **Neutral**: Higher dependency on a working LSP server for optimal performance.
