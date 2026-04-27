# Tasks: Hybrid Search (v3.5)

## Phase 1: Foundation
- [x] `internal/types/types.go`: Add `MemoryInsight` and `HybridSearchResult` structs as defined in the design.
  - Success: Code compiles and types are available for import.

## Phase 2: Core Implementation
- [x] `internal/store/store.go`: Implement `getMemoryInsights` helper function.
  - Logic: Call `engram search <query> --project <repo> --limit 5`, parse output for ID, Type, Title, Why, and Learned.
  - Success: Function returns a slice of `MemoryInsight` for a given query.

## Phase 3: Wiring & Integration
- [x] `internal/mcp/handlers.go`: Add `HybridSearchParams` struct and `handleHybridSearch` function.
  - Logic: Parallel execution of `store.SearchSymbols` and `getMemoryInsights`. Aggregate results.
  - Success: Handler returns a combined JSON result.
- [x] `internal/mcp/server.go`: Register the `scouter_hybrid_search` tool in `registerTools`.
  - Success: Tool appears in MCP tool list.

## Phase 4: Testing & Verification
- [x] Manual Verification: Run `scouter_hybrid_search` for a known symbol with bugfix history (e.g., "Run").
  - Success: Output contains both symbol metadata and Engram insights.
- [x] `just build`: Verify project integrity.
  - Success: Build exit code 0.
