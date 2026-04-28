# ✅ TASKS: Adversarial Judge (scouter_judge)

## Phase 1: Foundation (Prompts & Utils)
- [ ] `internal/mcp/prompts.go`: Add `JudgeSystemPrompt` (Cynical Adversarial Review).
- [ ] `internal/utils/utils.go`: Implement `ParseRating(text string) (float64, error)` to extract ratings from judge outputs.
- [ ] `internal/utils/utils.go`: Implement `ExtractList(text string, header string) []string` for findings extraction.

## Phase 2: Core Logic (Synthesizer)
- [ ] `internal/mcp/handlers.go`: Define `JudgeParams` and `JudgeResult` structs.
- [ ] `internal/mcp/handlers.go`: Implement `handleJudge`.
    - [ ] Launch two parallel goroutines for Sampling.
    - [ ] Use `req.Session.CreateMessage`.
    - [ ] Perform synthesis (Average rating, divergence check).

## Phase 3: Wiring
- [ ] `internal/mcp/server.go`: Register `scouter_judge` tool.
- [ ] `internal/mcp/server_test.go`: Update tool count (to 20).

## Phase 4: Verification
- [ ] Manual Verification: Run `scouter_judge` on a sample diff.
- [ ] `just build`: Ensure project integrity.
