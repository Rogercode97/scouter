# Proposal: Impact Analysis Engine (Omniscience Edition)

## Intent

Enable AI agents and developers to perform deep, **Entity-level** impact analysis of code changes with **Predictive Risk Scoring**. By tracing callers recursively across the call graph and calculating structural importance, Scouter will answer the critical question: "If I modify this entity, what is the risk score and which parts of the system are affected?". This transforms Scouter from a lookup tool into an architectural oracle that surfaces high-risk changes before they are committed.

## Scope

### In Scope
- **Database Schema Update**: Add `link_type`, `callee_path`, and `centrality` to the `calls` and `symbols` tables to support rich relationship metadata and risk calculations.
- **Store Implementation**: Implementation of `GetImpact(ctx, symbolName, symbolPath, depth)` using a **Recursive Common Table Expression (CTE)** in SQLite for high-performance graph traversal.
- **Risk Scoring System**: Implementation of a 0.0 to 1.0 Risk Score based on centrality (indegree), transitive impact size, and public API exposure.
- **Visual Impact Mapping**: Automatic generation of **Mermaid.js** code to visualize the blast radius.
- **MCP Tool**: Definition of `scouter_impact` tool in the MCP server returning structured, risk-ranked impact data.

### Out of Scope
- **Cross-project Impact Analysis**: Initially limited to the current project/workspace context.
- **Dynamic Dispatch Resolution**: Advanced interface/trait implementation tracing (deferred to future work).
- **Historic Fragility**: Risk scoring based on Git history/failures (future integration).

## Capabilities

### New Capabilities
- `impact-analysis`: Provides recursive entity-level tracing with **0.0-1.0 Risk Scoring**.
- `visual-impact`: Generates Mermaid.js graph code for the identified blast radius.

### Modified Capabilities
- `call-indexing`: Updated to capture entity metadata, `link_type`, and `callee_path`.

## Approach

### 1. Database Evolution (B-Tree Precision)
The `calls` and `symbols` tables will be updated to prioritize B-tree indexed precision over FTS5 for symbol resolution:
- `link_type`: Categorizes the relationship (e.g., `static`, `dynamic`, `internal`, `external`).
- `callee_path`: The absolute path where the called symbol is defined.
- `indegree`: A pre-calculated count of unique callers per symbol for O(1) centrality lookup.

### 2. Risk Scoring Formula
The Risk Score ($R$) for an entity is calculated as:
$$R = \min(1.0, (C \times W_c) + (B \times W_b) + E)$$
Where:
- $C$: Centrality (number of unique callers).
- $B$: Blast Radius (total number of transitive dependents found in the traversal).
- $E$: Exported/Public API bonus (e.g., +0.2).
- $W$: Weights calibrated for the Go ecosystem.

### 3. Recursive CTE with Rank Fusion (RRF) Ready
A SQLite Recursive CTE will traverse the graph. Results are prepared for **Reciprocal Rank Fusion (RRF)** to allow future integration with semantic/vector search.
```sql
WITH RECURSIVE impact AS (
    -- Anchor: Immediate callers of the target
    SELECT caller_name, path as caller_path, 1 as depth
    FROM calls
    WHERE callee_name = ? AND callee_path = ?
    
    UNION
    
    -- Recursive Step: Callers of the callers
    SELECT c.caller_name, c.path, i.depth + 1
    FROM calls c
    JOIN impact i ON c.callee_name = i.caller_name AND c.callee_path = i.caller_path
    WHERE i.depth < ?
)
SELECT DISTINCT caller_name, caller_path, depth FROM impact;
```

### 4. Visual Output (Mermaid)
The `scouter_impact` tool will include a `mermaid` field in its JSON response, providing a ready-to-render graph of the impact chain (e.g., `A --> B --> C`).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/store/store.go` | Modified | Add `GetImpact` with Recursive CTE and Risk Scoring logic. |
| `internal/store/store_test.go` | Modified | Unit tests for recursion, cycles, and risk calculation. |
| `internal/engine/parser.go` | Modified | Update AST traversal to capture precise entity metadata and `CalleePath`. |
| `internal/types/types.go` | Modified | New `ImpactResult` and `ImpactEntity` structs including `RiskScore`. |
| `cmd/scouter/main.go` | Modified | MCP tool registration with `mermaid` output support. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Infinite Recursion | Low | SQLite `UNION` (deduplication) + hard `depth` limit of 10. |
| Performance Hit | Medium | Compound B-tree index on `(callee_name, callee_path)`. |
| Path Resolution Overload | Medium | Implement caching for `filepath.EvalSymlinks` to mitigate syscall overhead. |

## Success Criteria
- [ ] `scouter_impact` returns a 0.0-1.0 Risk Score for any symbol.
- [ ] Visual Mermaid code is generated for impact chains > 1 level.
- [ ] Disambiguation correctly identifies the right "Bar" when two exist in different files.
- [ ] Rating 10.0 performance: Impact analysis < 100ms for projects < 50k lines.

## Future Implementations
- **Hybrid Search Fusion**: Blending keyword impact with semantic similarity using RRF.
- **CI/CD Oracle**: Automatically running tests based on the identified Blast Radius.

