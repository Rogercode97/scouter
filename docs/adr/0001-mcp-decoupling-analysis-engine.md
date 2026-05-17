# ADR-0001: Decoupling Transport Layer via AnalysisEngine

## Status
Accepted

## Context
Scouter's core logic was tightly coupled to the MCP protocol in `internal/mcp/handlers.go`. This "shallow" architecture forced business logic, transaction management, and I/O handling into the transport layer, making it difficult to reuse logic for the CLI or to test the engines in isolation.

## Decision
We have introduced an **AnalysisEngine** in `internal/engine/truth.go` as a core module that orchestrates all main operations.
- The `AnalysisEngine` encapsulates the `Store`, `LSPManager`, and specialized engines.
- We introduced a `Messenger` interface to decouple LLM requests from the MCP protocol.
- All transport-specific handlers (MCP) must now act as thin adapters that delegate to the `AnalysisEngine`.

## Consequences
- **Positive**: Improved locality of business logic. Core logic is now testable without an MCP session.
- **Negative**: Adds a small amount of boilerplate for new tools as they must be defined in both the Engine and the Handler.
- **Neutral**: Transaction management moved from handlers to the Engine core.
