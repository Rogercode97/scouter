# 📝 SPECIFICATION: Divine Linker (Wave 11)

## 1. Overview
This document specifies the behavior for semantic interface resolution and LSP lifecycle management in Scouter V8.0 (Wave 11 Ascension).

## 2. Requirements

### Requirement: REQ-LINKER-1 - Semantic Interface Resolution
The system MUST use LSP `textDocument/implementation` to identify concrete implementors of an interface.
**Context**: Nominal matching is unreliable. Semantic resolution via LSP provides compiler-grade truth.

#### Scenario: Resolve implementations for a Go interface
**GIVEN** an interface `Repository` defined in `internal/store/store.go`
**WHEN** the Dynamic Linker is triggered
**THEN** it MUST query the LSP server at the interface's position
**AND** it MUST record an "implements" link in the Call Graph for every returned implementation location

### Requirement: REQ-LINKER-2 - Decoupled Architecture
The resolution logic MUST live in the `engine` layer and interact with the `store` via its public interface.
**Context**: Prevents circular dependencies and adheres to Hexagonal mandates.

### Requirement: REQ-LSP-3 - Deterministic Shutdown
The MCP Server MUST explicitly terminate all LSP subprocesses upon exit.
**Context**: Wave 11 requires zero orphaned processes.

#### Scenario: Clean shutdown on SIGINT
**GIVEN** the Scouter MCP server is running with active `gopls` subprocesses
**WHEN** a SIGINT signal is received
**THEN** the server MUST call `lspMgr.Close()`
**AND** all LSP subprocesses MUST be terminated before the main process exits

## 3. Constraints
- **LSP Timeout**: LSP queries MUST NOT exceed 2 seconds to prevent blocking the indexer.
- **Batch Transaction**: All links found during a Linker Strike MUST be saved in a single database transaction.
