# Design: Sovereign Orchestration (Scouter 🤝 Engram)

## Technical Approach

Integrate persistent memory (Engram) directly into Scouter's autonomous loops (Compaction, Self-Healing, and Judgment) through hybrid prompt mandates and context priming via CLI execution.

## Architecture Decisions

### Decision: Context Fetching Location

**Choice**: Fetch Engram context within MCP Handlers (`internal/mcp/handlers.go`) using `exec.CommandContext("engram", "search")`.
**Alternatives considered**: Extending `TruthEngine` to manage Engram API/DB directly.
**Rationale**: Keeps `TruthEngine` decoupled from Engram's internal implementation. Scouter already interacts with Engram via CLI in `handleSaveAnchor`.

### Decision: Prompt Mandate vs Explicit Tool Use

**Choice**: Inject Engram context directly into the prompt payload before sampling.
**Alternatives considered**: Providing the Engram search tool to the Agent via MCP.
**Rationale**: Agents might ignore the tool or fail to construct the right query. Pre-fetching guarantees Sovereign constraints are applied to every operation.

### Decision: Token Safety Limits

**Choice**: Hard limit injected Engram history to top 3 results and truncate text to 1000 characters total.
**Alternatives considered**: Passing full context or dynamic chunking.
**Rationale**: Prevents token bloat, ensuring core operations remain fast and cheap.

## Data Flow

    Handler (SelfHeal/Judge) ──→ Engram CLI (search)
             │                          │
             └──────────────┐           │ (Top 3 Insights)
                            ▼           ▼
                   Prompt Construction (Context + Task)
                            │
                            ▼
                    MCP Sampling (LLM)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/mcp/prompts.go` | Modify | Update `CompactContextSystemPrompt` to enforce Engram schema (`**What**`, `**Why**`, `**Where**`, `**Learned**`). Add historical context mandates to `JudgeSystemPrompt` and `SelfHealSystemPrompt`. |
| `internal/mcp/handlers.go` | Modify | Update `handleSelfHeal` and `handleJudge` to execute `engram search` based on the context and inject the truncated results into the sampling prompt. |

## Interfaces / Contracts

No new interfaces. `engram search` outputs will be parsed natively or injected as raw strings. Token limits will be enforced using substring truncation.

```go
func fetchEngramContext(ctx context.Context, query string) string {
    // executes engram search, limits to 3 results, truncates to 1000 chars
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Token Limits | Verify `fetchEngramContext` correctly truncates over-limit text. |
| Integration | Handler Priming | Mock `engram` CLI, verify prompt string contains injected context. |

## Migration / Rollout

No migration required.

## Open Questions

- None
