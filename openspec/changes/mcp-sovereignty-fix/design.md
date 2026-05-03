# Design: MCP Server Sovereignty Fix

## Status
- **Proposed**: 2025-05-24
- **Project**: Scouter
- **Context**: Wave 11.1 Sovereignty Protocol

## Problem Statement
The current MCP implementation suffers from three primary issues:
1. **Sampling Fragility**: Clients that don't support `sampling/createMessage` cause critical tools (Judge, Evolve) to fail with cryptic RPC errors.
2. **Lack of Dry-Run Safety**: Mutation tools execute changes directly on disk without a preview mechanism, violating AI safety boundaries.
3. **Context Inefficiency**: Architectural metadata (Call Graph, tool schemas) isn't exposed as resources, forcing repeated expensive tool calls.

## Architecture Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| ADR-0003 | Graceful Sampling Fallback | Prevents tool crashes in unsupported clients while maintaining protocol integrity. |
| ADR-0004 | Disk-Backed Staging Ledger | Allows "dry-run" previews to survive server restarts, matching the stateful nature of complex refactors. |
| ADR-0005 | Truncated Resource Streams | Exposes massive graphs as resources without hitting MCP message size limits (25k chars). |

## Data Flow (ASCII)

```text
[Client] --(Call Tool: evolve{dryRun:true})--> [MCP Server]
                                                    |
[Ledger] <--(Stage Mutations)-----------------------|
    |                                               |
    |--(Generate Unified Diff)--------------------->|
                                                    |
[Client] <--(Unified Diff Result)-------------------|

[Client] --(Read Resource: dependencies)----------> [MCP Server]
                                                    |
[TruthEngine] <--(Query Global Call Graph)----------|
    |                                               |
    |--(Truncate/Summarize)------------------------>|
                                                    |
[Client] <--(Summarized Graph Resource)-------------|
```

## Implementation Strategy

### 1. Sampling Fallback (`internal/mcp/messenger.go`)
- Wrap `CreateMessage` calls in a recovery block.
- Detect JSON-RPC error code `-32601` (Method not found).
- Return a standard fallback string: `⚠️ MCP Sampling unsupported by client. Please review the proposal manually.`

### 2. Staging Ledger (`internal/engine/ledger.go`)
- **Diff Generation**: Implement a simple line-based diff using a library or manual comparison.
- **Persistence**: Add `SaveStaging()` and `LoadStaging()` using JSON serialization to `.scouter/staging/`.
- **Dry-Run Parameter**: Update `EvolveParams` and `RippleRefactorParams` to include `DryRun bool`.

### 3. Static Resources (`internal/mcp/resources.go`)
- **Dependency Graph**: Serialize top 100 central symbols and their edges.
- **MCP Schema**: Iterate over `mcpServer.Tools` to generate a JSON representation of registered tool definitions.

## Testing Strategy

### Unit Tests
- `internal/engine/ledger_test.go`: Test diff generation, staging persistence, and reload.
- `internal/mcp/messenger_test.go`: Mock a failing client and verify the fallback message.

### Integration Tests
- `tests/dry_run_test.go`: Execute `ripple_refactor --dry-run` and verify disk remains unchanged while a diff is returned.
- `tests/resource_test.go`: Verify `file:///scouter/graph/dependencies` returns valid content.

## Risk Assessment
- **State Desync**: If files change on disk after staging but before commit. *Mitigation*: Diff generation should re-validate disk content.
- **Memory Pressure**: Large diffs or graphs. *Mitigation*: Aggressive truncation and summarizing.
