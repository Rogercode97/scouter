# ADR-0001: Decoupling Transport Layer via TruthEngine

## Status
Accepted

## Context
Scouter's core logic was tightly coupled to the MCP protocol in `internal/mcp/handlers.go`. This "shallow" architecture forced business logic, transaction management, and I/O handling into the transport layer, making it difficult to reuse logic for the CLI or to test the engines in isolation.

## Decision
We have introduced a **TruthEngine** in `internal/engine/truth.go` as a deep module that orchestrates all core operations.
- The `TruthEngine` encapsulates the `Store`, `LSPManager`, and specialized engines.
- We introduced a `Messenger` interface to decouple LLM requests (Sampling/Oracle) from the MCP protocol.
- All transport-specific handlers (MCP) must now act as thin adapters that delegate to the `TruthEngine`.

## Consequences
- **Positive**: Improved **Locality** of business logic. High **Leverage** for any new transport added. Core logic is now testable without an MCP session.
- **Negative**: Adds a small amount of boiler-plate for new tools as they must be defined in both the Engine and the Handler.
- **Neutral**: Transaction management moved from handlers to the Engine core.
