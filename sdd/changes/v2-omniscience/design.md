# DESIGN: LSP Omniscience (Phase 1) - Wave 12.0

## Gap Analysis
*   **Current State**: Scouter uses tree-sitter AST and regex to find symbol references and callers. This is fast but heuristic.
*   **Target State**: Use `textDocument/prepareCallHierarchy` and `callHierarchy/incomingCalls` for deterministic tracking.

## Impacted Symbols
*   `internal/engine/lsp/client.go`: `LSPClient` (Modify to add CallHierarchy methods).
*   `internal/engine/lsp/types.go`: Add LSP call hierarchy structures.
*   `internal/engine/ripple.go`: Update `CalculateImpact` logic.
*   `internal/mcp/handlers.go`: Update `scouter_callers` and `scouter_impact` tool handlers.

## Architectural Alternatives (Tree-of-Thought)

### 1. Minimalist (Overlay)
*   Add LSP calls as a separate "debug" tool. No core change.
*   **Pro**: Low risk. **Con**: No synergy with existing tools.

### 2. Hybrid Determinism (Recommended)
*   Integrate LSP data into the existing `ImpactEngine`. Use LSP as the primary source of truth; fall back to AST if LSP is unavailable.
*   **Pro**: Zero false positives. Preserves speed for non-indexed symbols.
*   **Con**: Increased complexity in the Linker.

### 3. Pure Graph (Refactor-heavy)
*   Abandon AST heuristics and move everything to a Global Call Graph stored in SQLite.
*   **Pro**: Maximum performance after initial index.
*   **Con**: Massive refactor. High memory usage for huge repos.

## Choice: Hybrid Determinism (2)
We will leverage LSP for high-fidelity confirmation while keeping AST for fast discovery.
