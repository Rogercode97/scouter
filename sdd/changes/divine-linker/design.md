# 📐 TECHNICAL DESIGN: Divine Linker & LSP Stability (Wave 11)

## 1. Technical Approach
Elevate Scouter to Wave 11 standards by implementing a semantic **Dynamic Linker** that replaces nominal $O(N^2)$ matching with LSP-verified interface resolution. We will also enforce deterministic lifecycle management for LSP subprocesses to eliminate resource leaks identified in the Supreme Judgment.

## 2. Architecture Decisions

### Decision 1: Inverted Dependency Injection (Linker Engine)
- **Choice**: Extract `ResolveInterfaces` logic from `internal/store/store.go` into a new `internal/engine/linker.go`.
- **Rationale**: Decouples persistence from analysis. `store` provides the data, `engine` provides the intelligence. This prevents circular dependencies between `store` and `engine/lsp`.

### Decision 2: LSP-Backed Semantic Linking
- **Choice**: Use the `textDocument/implementation` LSP method to resolve interface satisfaction.
- **Rationale**: Moves from "Vibe Matching" (string signatures) to "Compiler Truth". Ensures 100% fidelity in the Global Call Graph.

### Decision 3: Deterministic Lifecycle (The Kill Switch)
- **Choice**: Implement `Server.Close()` in the MCP handler and `lsp.Manager.Close()` for explicit process termination.
- **Rationale**: Wave 11 mandates Zero-Slop resource management. `runtime.AddCleanup` is a safety net, but explicit closure is the sovereign path for long-running agents.

## 3. Data Flow
1. **Index Event**: File is parsed and symbols stored.
2. **Linker Strike**: `engine.LinkInterfaces(ctx, store, lspMgr)` is invoked.
3. **LSP Query**: For each `interface` symbol, the Linker queries the LSP for concrete implementations.
4. **Graph Update**: Linker batches "implements" edges into the `calls` table in a single transaction.

## 4. File Changes

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/engine/linker.go` | **Create** | New home for semantic interface resolution logic. |
| `internal/mcp/server.go` | **Modify** | Add `Close()` method to terminate `lspMgr`. |
| `cmd/scouter/main.go` | **Modify** | Capture signals and call `server.Close()` for clean exit. |
| `internal/store/store.go` | **Modify** | Remove `ResolveInterfaces` (deprecated) and add `SaveLink` helper. |
| `internal/mcp/handlers.go` | **Modify** | Update `handleIndex` to call the new `engine.LinkInterfaces`. |

## 5. Interfaces / Contracts

```go
// internal/engine/linker.go
func LinkInterfaces(ctx context.Context, repo store.Repository, lspMgr *lsp.Manager) error

// internal/mcp/server.go
func (s *Server) Close() error
```
