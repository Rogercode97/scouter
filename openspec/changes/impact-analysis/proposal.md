# Proposal: Impact Analysis Engine

## Intent

Enable AI agents and developers to perform deep impact analysis of code changes. By tracing callers recursively across the call graph, Scouter will answer the critical question: "If I modify this symbol, which parts of the system are potentially affected?". This reduces the risk of breaking changes and helps in identifying missing tests for impacted areas.

## Scope

### In Scope
- **Database Schema Update**: Add `link_type` (TEXT) and `callee_path` (TEXT) columns to the `calls` table to support rich relationship metadata and precise disambiguation.
- **Store Implementation**: Implementation of `GetImpact(ctx, symbolName, symbolPath, depth)` using a Recursive Common Table Expression (CTE) in SQLite for high-performance graph traversal.
- **MCP Tool**: Definition of `scouter_impact` tool in the MCP server to expose this capability to agents.
- **Path-based Disambiguation**: Logic to uniquely identify symbols when multiple definitions exist with the same name across different files/packages.

### Out of Scope
- **Cross-project Impact Analysis**: Initially limited to the current project/workspace context.
- **Dynamic Dispatch Resolution**: Advanced interface/trait implementation tracing (deferred to future work).
- **Impact Visualization**: Generating Mermaid graphs for impact (focused on data retrieval first).

## Capabilities

### New Capabilities
- `impact-analysis`: Provides recursive caller tracing to determine the ripple effect of symbol changes.

### Modified Capabilities
- `call-indexing`: Updated to capture `link_type` and `callee_path` during the indexing phase.

## Approach

### 1. Database Evolution
The `calls` table will be updated to include:
- `link_type`: Categorizes the relationship (e.g., `static`, `dynamic`, `internal`, `external`).
- `callee_path`: The absolute path where the called symbol is defined. This is crucial for the Recursive CTE to avoid "circular" paths or incorrect branches caused by name collisions.

### 2. Recursive CTE Logic
A SQLite Recursive CTE will be used to traverse the `calls` table upwards (from callee to callers).
```sql
WITH RECURSIVE impact AS (
    -- Anchor: Immediate callers of the target
    SELECT caller_name, path as caller_path, 1 as depth
    FROM calls
    WHERE callee_name = ? AND callee_path = ?
    
    UNION ALL
    
    -- Recursive Step: Callers of the callers
    SELECT c.caller_name, c.path, i.depth + 1
    FROM calls c
    JOIN impact i ON c.callee_name = i.caller_name AND c.callee_path = i.caller_path
    WHERE i.depth < ?
)
SELECT DISTINCT caller_name, caller_path, depth FROM impact;
```

### 3. Disambiguation Strategy
If `scouter_impact` is called with a `symbolName` that matches multiple symbols:
1. If `symbolPath` is provided, use it to filter the anchor.
2. If `symbolPath` is omitted, query the `symbols` table. If multiple definitions exist, return a "Multiple definitions found" error with a list of available paths to allow the agent to refine the request.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/store/store.go` | Modified | Add `GetImpact` method and update `migrate()` for `calls` schema. |
| `internal/store/store_test.go` | Modified | Unit tests for recursive CTE logic and disambiguation. |
| `internal/engine/parser.go` | Modified | Update `ASTCall` extraction to (optionally) resolve `callee_path`. |
| `internal/types/types.go` | Modified | Update `ASTCall` and `Call` structs to include new fields. |
| `cmd/scouter/main.go` | Modified | Register `scouter_impact` MCP tool and `ImpactRequest` validation. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Infinite Recursion | Low | SQLite handles basic cycles, but we enforce a hard `depth` limit (default 3, max 10). |
| Performance Hit | Medium | Recursive CTEs can be heavy. We will add an index on `(callee_name, callee_path)`. |
| Stale Index | High | If a file is modified but not re-indexed, `callee_path` might be wrong. The tool will warn if file hashes don't match. |

## Rollback Plan
1. Revert `internal/store/store.go` migration logic.
2. The `calls` table columns can remain (backward compatible) or be removed via a temporary migration script if needed.
3. Revert MCP tool registration in `main.go`.

## Dependencies
- SQLite 3.8.3+ (Required for CTE support, already covered by `modernc.org/sqlite`).

## Success Criteria
- [ ] `scouter_impact` returns correct recursive callers for a known function.
- [ ] Disambiguation correctly identifies the right "Bar" when two exist in different files.
- [ ] The engine respects the `depth` constraint to prevent context bloat.

## Future Implementations
- **Interface-to-Implementation Tracing**: Tracing impact through interface methods by resolving implementations.
- **Dead Code Cleanup integration**: Using impact analysis to safely suggest removals.
- **Impact-based Test Prioritization**: Automatically running tests for symbols in the impact set.
- **Visual Impact Maps**: Exporting impact data to Mermaid.js for architectural reviews.
