# Tasks: mcp-sovereignty-fix

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 150-200 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Not needed |
| Delivery strategy | ask-on-risk |

Decision needed before apply: Yes
Chained PRs recommended: No
400-line budget risk: Low

## Phase 1: Foundation / Infrastructure

- [x] 1.1 In `internal/mcp/handlers.go`, create `fetchEngramContext(query string)` that runs `engram search`, limiting to top 3 results and max 1000 characters.

## Phase 2: Core Implementation (Prompts)

- [x] 2.1 In `internal/mcp/prompts.go`, update `CompactContextSystemPrompt` to demand strict Engram output (`**What**`, `**Why**`, `**Where**`, `**Learned**`).
- [x] 2.2 In `internal/mcp/prompts.go`, update `JudgeSystemPrompt` to accept `historicalContext` and evaluate findings against it.
- [x] 2.3 In `internal/mcp/prompts.go`, update Healer prompt template to accept `historicalFixes` and instruct consideration of past patterns.

## Phase 3: Integration / Wiring

- [x] 3.1 In `internal/mcp/handlers.go`, update `handleJudge` to fetch ADR context via `fetchEngramContext` and inject it into the Judge prompt.
- [x] 3.2 In `internal/mcp/handlers.go` (or relevant TruthEngine integration), fetch historical bugfixes via `fetchEngramContext` and inject into the Healer prompt.