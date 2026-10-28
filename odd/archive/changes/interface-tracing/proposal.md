# 🛡️ PROPOSAL: Interface Tracing (LSP-Enhanced)

## 🎯 1. INTENT & RCA
- **Goal**: Enable true "Omniscience" for Scouter V2.0 by tracing interface implementations across the codebase.
- **RCA (Interface Blindness)**: The current static AST-based Call Graph can only trace direct function calls. It fails to resolve dynamic dispatches through interfaces, leaving blind spots in the impact analysis of decoupled architectures.
- **Value**: Guaranteed deterministic impact analysis (blast radius) for decoupled Go codebases, elevating Scouter to a sovereign truth kernel.

## 🧱 2. SCOPE
### ✅ IN SCOPE
- **Sovereign Enricher Pass**: A post-index phase that queries the LSP for interface implementations.
- **LSP Protocol Expansion**: Implementation of the `textDocument/implementation` method in the internal LSP client.
- **Store Evolution**: Schema/Store updates to persist `implements` or `resolved_call` relationships in the SQLite database.

### ❌ OUT OF SCOPE
- **Live/Dynamic Tracing**: No runtime instrumentation or tracing (ebpf/ptrace).
- **Multi-language Support (Initially)**: Focused solely on Go 1.24+ gopls capabilities for now.

## ⚡ 3. CAPABILITIES (CONTRACT)
| Capability | Type | Description |
| :--- | :--- | :--- |
| `lsp-implementation-query` | NEW | Expose `textDocument/implementation` in the LSP client. |
| `enrich-call-graph` | NEW | Run a post-index pass to link interface calls to concrete implementations. |
| `scouter-impact-interfaces` | MODIFIED | `scouter_impact` must seamlessly traverse interface-to-implementation boundaries. |

## 🗺️ 4. AFFECTED AREAS (BLAST RADIUS)
Based on `scouter_impact` analysis for `Store`:
- `internal/engine/lsp/client.go` (Distance: 1 - 2)
- `internal/store/store.go`
- `cmd/index-vault/main.go`

## 🗡️ 5. APPROACH: "SOVEREIGN ENRICHER"
1. **Index Phase (Unchanged)**: AST parses files and builds the initial static Call Graph.
2. **Enrichment Phase (New)**: 
   - Identify interface definitions in the Store.
   - For each interface, use the LSP Client to call `textDocument/implementation`.
   - Map returned locations back to Store symbols.
   - Insert new edges (`link_type: implements` or `link_type: resolved_call`) into the Call Graph.
3. **Query Phase (Modified)**: `scouter_impact` transparently traverses both `call` and `implements` edges.

## ⏪ 6. ROLLBACK PLAN
- **Database**: Revert the schema or ignore `implements` link types in queries.
- **Enricher**: Disable the post-index phase via a feature flag (`--disable-enrichment`) to fall back to the static AST call graph.
