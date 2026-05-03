## Exploration: MCP Sovereignty Fix

### Current State
1. **Sampling/Judge Error**: The `scouter_judge` tool and others that use `sampling/createMessage` (like `evolve`) fail with `-32601 Method not found`. This occurs because the host client (e.g., Gemini CLI, Cursor, Claude Desktop) does not currently implement or expose the `sampling` capability handlers. The MCP server tries to ask the client to run a prompt, but the client rejects the RPC call.
2. **Staging Ledger (Dry-Run)**: Mutation tools (`evolve`, `ripple_refactor`) execute changes directly on disk. Although `internal/engine/ledger.go` has `Stage` and `CommitStaged` methods, there is no MCP mechanism to preview these changes before they are committed. This violates the safety boundaries of an AI agent.
3. **Static Resources**: The Dependency Graph and MCP Schema are not currently exposed as read-only resources, which forces the agent to repeatedly query tools or guess the architecture, wasting context.

### Affected Areas
- `internal/mcp/handlers.go` — Tools relying on `CreateMessage` (Judge, Evolve, CompactContext).
- `internal/engine/ledger.go` — Needs diff generation logic for staged patches.
- `internal/mcp/resources.go` — Missing endpoints for static context (dependency graph, schema).

### Approaches

#### 1. Sampling Error Fallback
- **Approach**: Since we cannot force the client to support `sampling/createMessage`, we must degrade gracefully. We can introduce a "Mock/Manual Review" fallback or simply return a clear instruction in the tool result that Sampling is unsupported by the current client.
- **Pros**: Prevents the tool from crashing with a cryptic JSON-RPC error.
- **Cons**: We lose the autonomous adversarial review loop until the client supports it.
- **Effort**: Low

#### 2. Staging Ledger (Dry-Run)
- **Approach**: 
  - Enhance `Ledger` to generate a unified diff string of all staged patches.
  - Modify `ripple_refactor` and `evolve` to accept a `dryRun: true` parameter (or create separate tools like `evolve_plan` / `evolve_apply`). When in dry-run mode, stage the patches, generate the diff, and return the diff as the tool output. 
  - Create a new read-only resource `file:///scouter/staging/diff` to allow the agent to inspect the current staged changes before committing.
- **Pros**: Provides a safety net, enabling the agent to visually inspect architectural changes before applying them. Matches the "Sovereignty" and "Dry-Run" ideals.
- **Cons**: Requires maintaining the staged state in memory between tool calls (stateful server).
- **Effort**: Medium

#### 3. Static Resources Mapping
- **Approach**: 
  - Add `file:///scouter/graph/dependencies` to `resources.go` which serializes the Global Call Graph from `store.Repository`.
  - Add `file:///scouter/schema/mcp` to expose the JSON schema of registered tools.
- **Pros**: Maximizes Resource Sovereignty, reducing tool-call bloat and providing instant context.
- **Cons**: Serializing a massive dependency graph could be large, might need pagination or careful truncation.
- **Effort**: Low

### Recommendation
- **Sampling**: Add a capability check or graceful error handling in `mcpMessenger.Ask` and the handlers. Provide a fallback that instructs the user to review the proposal manually if sampling fails.
- **Staging Ledger**: Implement `dryRun` parameters in mutation tools. If `dryRun=true`, the tool stages the change in `Ledger`, returns a diff, and avoids disk writes. The agent can then call the tool with `dryRun=false` (or a `commit` tool) to apply.
- **Static Resources**: Register `file:///scouter/graph/dependencies` with a truncated or summarized output of the top central symbols to avoid breaking the 25k char limit.

### Risks
- **Statefulness**: Holding staged patches in memory means if the MCP server restarts between a `dryRun` and an `apply`, the staging area is lost. (Mitigation: we can serialize staged patches to `.scouter/staging/` to survive restarts).
- **Client Limitations**: Some clients might not show the full diff if it's too large, hence the need for a file resource or truncation.

### Ready for Proposal
Yes. The orchestrator can proceed to formulate the formal Change Proposal (`sdd-propose`) based on these findings.