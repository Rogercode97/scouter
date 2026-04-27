# 🛡️ SDD DESIGN: Go 1.25 Optimizations

## 1. Technical Approach
Adopt Go 1.25 native features (`runtime.AddCleanup` and `encoding/json/v2`) across core infrastructure to eliminate manual finalizer hooks, reduce serialization latency, and provide deterministic resource cleanup. Investigate AST traversal optimizations using `go/ast.PreorderStack` for language-specific parsing.

## 2. Architecture Decisions

### AD-01: Deterministic Lifecycle via `runtime.AddCleanup`
* **Choice**: Replace `defer` and manual `Close()` orchestration in persistent singletons with `runtime.AddCleanup`.
* **Alternatives**: Keep relying on manual context cancellation and explicit `.Close()` calls.
* **Rationale**: Go 1.25's `runtime.AddCleanup` guarantees execution without relying on `runtime.SetFinalizer` limitations, effectively preventing zombie LSP subprocesses in `Manager` and leaking database connections in `*sql.DB`.

### AD-02: High-Performance Serialization (`encoding/json/v2`)
* **Choice**: Migrate JSON unmarshaling/marshaling in `store.go` and `mcp/handlers.go` to `encoding/json/v2`.
* **Alternatives**: Use third-party fast json libraries (e.g., `goccy/go-json` or `sonic`).
* **Rationale**: `encoding/json/v2` provides zero-allocation parsing and significantly faster streaming native to the standard library, maintaining zero-slop dependency mandates while cutting down serialization CPU time in MCP communications.

### AD-03: Native Preorder AST Traversal
* **Choice**: Explore/implement a specialized Go AST parser using `go/ast.PreorderStack`.
* **Alternatives**: Continue using the generic Treesitter parser for `.go` files.
* **Rationale**: While Treesitter handles all languages uniformly, Go's native AST with `PreorderStack` (Go 1.25+) drastically reduces traversal overhead, which is critical for `StreamSymbols` performance in large Go codebases.

## 3. Data Flow

```ascii
+-----------------------+       [1] Create          +--------------------------+
|                       |  ---------------------->  |      Store / LSP         |
|   Caller / Init       |                           |      Manager             |
|                       |  <----------------------  | (attached runtime.       |
+-----------------------+       [2] Return Ref      |  AddCleanup hooks)       |
                                                    +------------+-------------+
                                                                 |
                                                                 | [3] GC Collects /
                                                                 |     Object Unreachable
                                                                 v
                                                    +--------------------------+
                                                    |  Cleanup Functions:      |
                                                    |  - DB.Close()            |
                                                    |  - cmd.Process.Kill()    |
                                                    +--------------------------+
```

## 4. File Changes (Impact-Verified)

| File | Action | Rationale (Blast Radius Analyzed) |
|------|--------|-----------------------------------|
| `internal/store/store.go` | **MOD** | Attach `runtime.AddCleanup` in `New()` for `*sql.DB`. Swap to `encoding/json/v2`. Impact: High Centrality. Guarantees safe connections. |
| `internal/engine/lsp/manager.go` | **MOD** | Attach `runtime.AddCleanup` upon LSP process spawn. Impact: Mitigates zombie processes during abrupt agent exits. |
| `internal/mcp/handlers.go` | **MOD** | Use `json/v2` for MCP JSON-RPC messages. Impact: Increases throughput on heavy AST query responses. |
| `internal/engine/parser.go` | **MOD** | Add fallback/override logic for `.go` files using `go/ast.PreorderStack` inside `StreamSymbols`. Impact: Localized to Go parsing. |

## 5. Interfaces / Contracts

```go
// Proposed change in json handling signature (conceptual)
import json "encoding/json/v2"

// Using json.Unmarshal instead of legacy encoding/json
func (h *MCPHandler) handleRequest(req []byte) error {
    var payload MCPPayload
    if err := json.Unmarshal(req, &payload); err != nil {
        return err
    }
    // ...
}

// Store initialization cleanup hook
func New(dbPath string) (*Store, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil { return nil, err }
    
    // Go 1.25 native cleanup
    cleanupObj := runtime.AddCleanup(db, func(db *sql.DB) {
        _ = db.Close()
    }, db)
    
    return &Store{db: db}, nil
}
```