# Design: Hybrid Search (v3.5)

## Technical Approach
Unify code-level symbols with project memory by executing parallel searches in the Scouter SQLite database (AST) and the Engram FTS5 database (Historical Context).

## Architecture Decisions

### 1. Unified Search Entry Point
- **Choice**: Create a new tool `scouter_hybrid_search` instead of modifying `scouter_search`.
- **Rationale**: Maintain backward compatibility for agents that only need code-level lookups while providing an advanced "Knowledge" option.

### 2. Information Mapping
- **Choice**: Cross-reference results using symbol names and file paths.
- **Rationale**: Provides the most reliable link between a physical line of code and a past architectural decision or bugfix recorded in Engram.

### 3. Memory Distillation (Pure Signal)
- **Choice**: Extract and return only the `Learned` and `Why` fields from Engram results.
- **Rationale**: Protects the AI context window from administrative metadata and focuses on technical wisdom.

## Data Flow
1. User Query -> `scouter_hybrid_search` tool handler.
2. Async Branch A: `store.SearchSymbolsWeighted` (AST intelligence).
3. Async Branch B: `engram search --project <current>` (Historical intelligence).
4. Reducer: Link AST symbols with memories based on string matching.
5. Filter: Distill memory content to "Pure Signal".
6. Result: Combined JSON returned to Gemini CLI.

## File Changes

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/types/types.go` | Modify | Define `HybridSearchResult` and `MemoryInsight` structs. |
| `internal/store/store.go` | Modify | Implement `SearchMemories` helper calling `engram` binary. |
| `internal/mcp/handlers.go` | Modify | Implement `handleHybridSearch` orchestration logic. |
| `internal/mcp/server.go` | Modify | Register `scouter_hybrid_search` tool. |

## Interfaces / Contracts

```go
type MemoryInsight struct {
    ID      string `json:"id"`
    Type    string `json:"type"`
    Title   string `json:"title"`
    Learned string `json:"learned"`
    Why     string `json:"why"`
}

type HybridSearchResult struct {
    Symbols  []Symbol        `json:"symbols"`
    Insights []MemoryInsight `json:"insights"`
}
```
