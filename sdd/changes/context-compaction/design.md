# Design: Context Compaction (v4.5)

## Technical Approach
Leverage MCP `sampling/createMessage` to trigger a self-summarization loop within the model, then persist the resulting "latent memory" to a hidden project directory.

## Architecture Decisions

### 1. Self-Summarization Loop
- **Choice**: Use the active model to summarize the history instead of an external process.
- **Rationale**: The active model has the most accurate "latent" understanding of the current work-in-progress and technical decisions.

### 2. File-Based Persistence
- **Choice**: Store the summary in `.scouter/anchor.md`.
- **Rationale**: Keeps the memory portable, easy for the agent to find in future sessions (via `ls`), and allows human review.

### 3. Sampling Prompt Engineering
- **Choice**: Use a high-density system prompt for sampling.
- **Rationale**: Ensures the model produces a "Pure Signal" summary (facts, code paths, state) instead of prose.

## Data Flow
1. Tool Call: `scouter_compact_context`.
2. Sampling Request: Scouter asks Model -> "Summarize our technical state".
3. Response Receipt: Scouter captures the Markdown summary.
4. Persistence: Write summary to `.scouter/anchor.md`.
5. Feedback: Return success message with token savings estimate.

## File Changes

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/mcp/handlers.go` | Modify | Implement `handleCompactContext` logic. |
| `internal/mcp/server.go` | Modify | Register `scouter_compact_context` tool. |

## Interfaces / Contracts

```go
type CompactContextParams struct {
    Force bool `json:"force,omitempty"`
}
```
