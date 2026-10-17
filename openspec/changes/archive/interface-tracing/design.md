# Interface Tracing (LSP-Enhanced) - Design Document

## Technical Approach
Extend the Scouter core to resolve dynamic interface dispatches by extending the LSP client to support `textDocument/implementation`. Introduce a dedicated `Enricher` component that coordinates DB querying of interfaces, resolving implementations via LSP, and safely persisting the dynamic edges using transaction limits to prevent OOM and context leaks.

## Architecture Decisions

### 1. Dedicated Enricher Component
- **Choice**: Create `internal/engine/enricher.go` as a standalone coordinator.
- **Alternatives**: Embed LSP implementation lookups directly inside the static indexer.
- **Rationale**: Strict separation of concerns. Indexing handles static AST state. Enrichment handles I/O-heavy dynamic cross-referencing and provides distinct boundaries for memory control and timeout contexts (Rating 10.0 adherence).

### 2. Transactional & Batched DB Inserts
- **Choice**: Persist interface links via `Store.WithTransaction` using bounded batch sizes and strict contexts.
- **Alternatives**: Individual fire-and-forget inserts or unlimited batch sizes.
- **Rationale**: Prevents OOM in large repositories and guarantees atomicity. Context propagation prevents goroutine leaks if the LSP server crashes or hangs.

### 3. Unified Call Graph Edge Type
- **Choice**: Store implementations in the existing `calls` table using `LinkType: dynamic`. 
- **Alternatives**: Introduce an `implementations` table.
- **Rationale**: Integrating into `calls` allows Scouter V2.0 Global Call Graph queries to naturally traverse interfaces to concrete implementations without complex SQL joins. 

## Data Flow

```text
[cmd/scouter] (Init via --enrich or auto post-index)
     |
     v
[Enricher Component] <--- (1) Fetch Interface Definitions
     |                 |  (internal/store)
     |                 v
     |-- (2) Iterative Context-Bound Loop
     |
     +-----> [LSP Client]
     |         |-- textDocument/implementation
     |         v
     |<----- [LSP Response (Locations)]
     |
     v
[Enricher Component] (3) Maps Caller (Interface.Method) -> Callee (Struct.Method)
     |
     v
[DB Store] (4) Batched Transactional Insert (`calls` table, LinkType: "dynamic")
```

## File Changes (Impact-Verified)

| File | Action | Rationale |
|---|---|---|
| `internal/engine/enricher.go` | Create | New coordinator component `Enricher{}`. Bound to `context.Context` to handle cancellation gracefully. |
| `internal/engine/lsp/client.go` | Modify | Expose `Implementation(ctx, ImplementationParams) ([]Location, error)` for `textDocument/implementation`. |
| `internal/store/store.go` | Modify | Provide capabilities to iterate over tracked interfaces and batch insert into `calls` table using a transaction. |
| `cmd/scouter/main.go` | Modify | Register `--enrich` CLI flag and trigger `enricher.Enrich(ctx)` after the indexing phase completes. |

## Interfaces / Contracts

```go
// internal/engine/enricher.go
type Enricher struct {
    store     Store
    lspClient LSPClient
}

// Enrich executes the end-to-end interface implementation resolution.
func (e *Enricher) Enrich(ctx context.Context) error

// internal/engine/lsp/types.go
// Contract for standard LSP textDocument/implementation request
type ImplementationParams struct {
    TextDocument TextDocumentIdentifier `json:"textDocument"`
    Position     Position               `json:"position"`
}

// Database Schema Mapping Rule for 'calls' table:
// Caller:   "InterfaceName.MethodName"
// Callee:   "StructName.MethodName"
// LinkType: "dynamic"
```
