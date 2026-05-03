# Proposal: MCP Sovereignty Fix

## Intent
Address critical failures in MCP Sampling when clients (e.g., Gemini CLI) do not support the protocol, introduce a safe staging mechanism for code mutations, and expose architectural resources to reduce context usage.

## Scope

### In Scope
- **Sampling Fallback**: Implement graceful error handling in `mcpMessenger.Ask` and handlers (`scouter_judge`, `evolve`, `compact_context`). Return manual review instructions instead of RPC errors.
- **Staging Ledger**: 
    - Add unified diff generation to `internal/engine/ledger.go`.
    - Persist staged patches to `.scouter/staging/` to survive server restarts.
- **Dry-Run Mode**: 
    - Add `dryRun` parameter to `ripple_refactor` and `evolve` tools.
    - If enabled, tools stage changes and return diffs without modifying source files.
- **Static Resources**:
    - `file:///scouter/graph/dependencies`: Summarized Global Call Graph.
    - `file:///scouter/schema/mcp`: MCP tool schema for agent discovery.

### Out of Scope
- Implementing client-side sampling support.
- Automated commit of staged patches without explicit agent directive.

## Capabilities

### New Capabilities
- `staging-ledger`: Provides atomic staging, diff generation, and persistence of the staging area.
- `static-resources`: Exposes architectural and protocol context as read-only MCP resources.

### Modified Capabilities
- `mcp-server`: Upgraded to handle sampling failures and expose new discovery endpoints.
- `ripple-engine`: Updated to support `dryRun` across all mutation handlers.

## Approach
1. **Sampling**: Detect `-32601` (Method not found) in `mcpMessenger.Ask`. Provide a standard fallback response that prompts manual intervention.
2. **Staging**: Implement a `Diff()` method in `Ledger` using a lightweight diffing strategy. Serialize `Staged` map to JSON in `.scouter/staging/`.
3. **MCP**: Register new resources in `internal/mcp/resources.go` and update tool signatures in `server.go` and `handlers.go`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/mcp/messenger.go` | Modified | Add Sampling fallback logic. |
| `internal/mcp/handlers.go` | Modified | Update handlers for `dryRun` and resource links. |
| `internal/mcp/resources.go` | Modified | Register dependency graph and schema resources. |
| `internal/engine/ledger.go` | Modified | Add diffing and staging persistence. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| State loss on restart | Medium | Disk persistence in `.scouter/staging/`. |
| Context bloat (resources) | Low | Summarize and truncate large outputs. |

## Rollback Plan
1. Delete `.scouter/staging/` to clear any corrupt staged state.
2. Revert `mcpMessenger` changes to restore direct (failing) sampling calls.
3. Revert `handlers.go` to remove `dryRun` parameters.

## Success Criteria
- [ ] `scouter_judge` returns "Manual Review Required" on sampling failure.
- [ ] `ripple_refactor --dry-run` returns a diff without modifying files.
- [ ] Staged patches survive an MCP server restart.
- [ ] MCP resources are accessible via `file:///scouter/` URI.
