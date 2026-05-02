# SPEC: LSP Omniscience (Phase 1) - Wave 12.0

## Status: DRAFT
## RFC 2119: MUST, SHOULD, MAY

### 1. Goal
Elevate Scouter's symbol analysis from heuristic regex/AST matching to deterministic call hierarchy tracking via LSP.

### 2. Scenarios (Gherkin)

#### Scenario: Deterministic Caller Tracking
*   **Given** an LSP server is active for the current project.
*   **When** a user requests `scouter_callers` for a specific symbol.
*   **Then** Scouter MUST query the LSP `callHierarchy/incomingCalls` method.
*   **And** it SHOULD return 100% accurate call locations verified by the language engine.

#### Scenario: Impact Analysis Verification
*   **Given** a symbol change is proposed.
*   **When** `scouter_impact` is executed.
*   **Then** it SHOULD use the LSP Call Graph to calculate the blast radius with zero false positives.

### 3. Requirements
1.  **REQ-1**: Implement `CallHierarchy` support in `internal/engine/lsp/client.go`.
2.  **REQ-2**: Implement `incomingCalls` and `outgoingCalls` wrappers.
3.  **REQ-3**: Update `internal/engine/ripple.go` or equivalent impact engine to prefer LSP data over AST heuristics when available.
4.  **REQ-4**: Add a fallback mechanism to AST heuristics if the LSP server does not support call hierarchy.
