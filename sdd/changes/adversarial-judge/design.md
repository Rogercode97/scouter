# 📐 TECHNICAL DESIGN: Adversarial Judge Engine (scouter_judge)

## 1. Technical Approach
Implement an MCP tool `scouter_judge` that automates the **Divine Verdict** protocol. It utilizes parallel MCP Sampling to launch two independent "Judge" agents that audit a proposed code change or architectural proposal. The results are synthesized into a final rating and a list of mandatory fixes.

## 2. Architecture Decisions

### Decision 1: Parallel Adversarial Sampling
- **Choice**: Execute two concurrent `req.Session.CreateMessage` calls with isolated contexts.
- **Rationale**: Guarantees **Information Asymmetry**. Judges must not see each other's work or the author's rationale beyond the diff/proposal.

### Decision 2: The "Hakaishin" Judge Prompt
- **Choice**: Use a strict, cynical system prompt that mandates identifying at least 3 risk vectors.
- **Rationale**: Prevents "Vibe Approvals" and ensures zero-slop scrutiny.

### Decision 3: Synthesized Divine Verdict
- **Choice**: If ratings diverge by > 2.0, the tool flags the verdict as "Contested" and requires a third senior audit (or human intervention).
- **Rationale**: Align with the Supreme Judgment protocol (Wave 8.9).

## 3. Data Flow
1. **Tool Call**: `scouter_judge(diff, proposal)`
2. **Branch A**: Launch Judge-A (Sampling)
3. **Branch B**: Launch Judge-B (Sampling)
4. **Synthesis**:
   - Parse Ratings (X.X/10.0)
   - Extract Risk Vectors
   - Calculate Average & Convergence
5. **Response**: Consolidated Divine Verdict Markdown/JSON.

## 4. File Changes

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/mcp/prompts.go` | **Modify** | Add `JudgeSystemPrompt`. |
| `internal/mcp/handlers.go` | **Modify** | Implement `handleJudge`. |
| `internal/mcp/server.go` | **Modify** | Register `scouter_judge` tool. |
| `internal/utils/utils.go` | **Modify** | Add `ParseRating` helper to extract "X.X/10.0" from text. |

## 5. Interfaces / Contracts

```go
type JudgeParams struct {
    Diff     string `json:"diff,omitempty"`
    Proposal string `json:"proposal,omitempty"`
}

type JudgeResult struct {
    Rating      float64  `json:"rating"`
    Verdict     string   `json:"verdict"` // SOVEREIGN, REDEMPTION, HAKAI
    Findings    []string `json:"findings"`
    RiskVectors []string `json:"risk_vectors"`
    Convergence bool     `json:"convergence"`
}
```
