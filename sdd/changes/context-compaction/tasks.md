# Tasks: Context Compaction (v4.5)

## Phase 1: Foundation
- [x] Define `CompactContextParams` and `CompactContextResult` structs.
- [x] Ensures type-safe parameter handling for the context window reduction logic.
- [x] Success: Structs exist and compile with `go build`.

## Phase 2: Logic
- [x] Implement `handleCompactContext` function using MCP Sampling.
- [x] Success: Function successfully requests a summary and receives a distilled technical state.

## Phase 3: Wiring
- [x] Register `scouter_compact_context` tool in the MCP server.
- [x] Success: Tool appears in `mcp_list_tools` output.

## Phase 4: Persistence
- [x] Implement `.scouter/anchor.md` state management.
- [x] Success: File is created/updated with the Markdown summary from the model.
