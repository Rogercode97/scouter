## Proposal: Dream Ascension: Passive Engram Capture (Wave 11.1)

### Intent
Automate the distillation and persistence of technical memories (ADRs, bug fixes, patterns) by implementing a passive, turn-end hook that utilizes a forked-agent pattern. This ensures Scouter maintains a high-fidelity Engram database without requiring manual user intervention or polluting project source files (GEMINI.md).

### Scope
- **Domain Logic**: Add `PassiveDistill` to `internal/domain/memory/service.go`.
- **Infrastructure**: Update `internal/adapters/llm/mcp_distiller.go` to support context-aware distillation prompts.
- **MCP Layer**: Implement a post-execution hook in `internal/mcp/handlers.go` to trigger background distillation.
- **Verification**: New integration tests ensuring background extraction doesn't interfere with main agent performance.

### Approach
1. **The Forked Trigger**: Hook into the end of successful tool-use sequences in the MCP server. If the task involved significant changes (writes, complex research), trigger the distillation.
2. **Context Sharing**: Pass the current session transcript to the `MCPDistiller` to utilize the existing prompt cache (inspired by Claude Code's `extractMemories.ts`).
3. **Engram Injection**: The distilled JSON (ADRs, Bugfixes, Patterns) will be converted into `engram.Observation` objects and saved via `memoryProvider.SaveObservation`.
4. **Mutual Exclusion**: Implement a simple guard to prevent extraction if the main agent has already manually saved an observation during the same turn.

### Expected Impact
- **Zero Friction**: Technical wisdom is captured automatically.
- **High Retention**: Eliminates the "memory loss" between sessions when users forget to run `/dream`.
- **Pure Source**: Keeps `GEMINI.md` clean for team-wide, high-level documentation only.

### Decision Registry
- **Storage**: Distilled memories go to Engram (SQLite), not GEMINI.md.
- **Concurrency**: Distillation runs in a non-blocking goroutine at the end of the turn.
- **Ki Budget**: Throttled to only run on "high-impact" turns (defined by token volume or file modifications).
