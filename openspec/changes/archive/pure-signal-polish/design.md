# 🛡️ Technical Design: Pure Signal Polish (Pre-V2.0)

## 1. Technical Approach
Enhance AST symbol resolution by incorporating full method signatures into the `Symbol` struct and SQLite store. Optimize pointer resolution by caching file hashes in the store instead of recalculating on every resolve, and introduce `context.Context` cancellation checks in the parsing engine for robust execution.

## 2. Architecture Decisions
- **Decision:** Include `Signature` in `Symbol` and `symbols` table.
  - **Choice:** Add `Signature string` to `Symbol` struct and SQLite schema.
  - **Alternatives:** Parse AST on every interface resolution.
  - **Rationale:** Storing the signature directly in the database allows for O(1) matching during interface resolution (`ResolveInterfaces()`) without re-parsing the file, heavily optimizing the global call graph preparation.
- **Decision:** Optimize `PointerResolver.Resolve()`.
  - **Choice:** Query `GetFileIndex(ctx, filePath)` and compare `idx.Hash` instead of computing `utils.CalculateHash()`.
  - **Alternatives:** Continue calculating hash on every pointer resolution.
  - **Rationale:** Recalculating the hash on every resolution is CPU intensive and redundant. Relying on the persisted file index hash provides significant performance gains and zero-slop execution.
- **Decision:** Engine Refinement with Tree-sitter.
  - **Choice:** Extract method signatures during parsing and respect `ctx.Done()`.
  - **Alternatives:** Only extract symbol names.
  - **Rationale:** Complete method signatures are necessary for accurate interface implementation checking (e.g., `MethodKey { Name, Signature }`). Checking `ctx.Done()` prevents goroutine leaks and ensures the process can be gracefully canceled, adhering to Go 1.24+ idioms.

## 3. Data Flow
```text
[Engine Parser] -> Extract AST (Name + Signature) -> [Store]
      |                                                |
      v                                                v
 Check ctx.Done()                               Insert into `symbols`
                                                (including `signature` col)

[Pointer Resolver] -> Query GetFileIndex() -> Compare Hash -> Resolve
[Interface Resolver] -> Group by MethodKey {Name, Signature} -> Map Implementations
```

## 4. File Changes
| File | Action | Rationale |
|------|--------|-----------|
| `internal/types/types.go` | Modify | Add `Signature string` to `Symbol` struct. Create `MethodKey` struct. |
| `internal/store/store.go` | Modify | Update `migrate()` to add `signature` column. Update `SearchSymbols` and `GetSymbolsByRange`. |
| `internal/store/dependency.go` | Modify | Refactor `ResolveInterfaces()` to use `MethodKey { Name, Signature }` for accurate matching. |
| `internal/engine/parser.go` | Modify | Extract parameters and return types via Tree-sitter. Add `ctx.Done()` checks in parsing loops. |
| `internal/engine/pipeline.go` (or Resolver context) | Modify | Update `PointerResolver.Resolve()` to use `GetFileIndex(ctx, filePath)` and compare hashes. Remove `utils.CalculateHash`. |

## 5. Interfaces / Contracts
```go
// internal/types/types.go
type Symbol struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Signature string `json:"signature"` // NEW: Full method signature (params + returns)
    Type      string `json:"type"`
    File      string `json:"file"`
    Line      int    `json:"line"`
    // ... existing fields
}

type MethodKey struct {
    Name      string
    Signature string
}
```
