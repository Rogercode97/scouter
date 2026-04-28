# 📝 SPECIFICATION: Adversarial Judge (scouter_judge)

## 1. Overview
This document specifies the behavior of the `scouter_judge` tool, which enforces the **Divine Verdict** mandate (Wave 11).

## 2. Requirements

### Requirement: REQ-JUDGE-1 - Dual Sampling
The tool MUST launch exactly two independent MCP Sampling requests to different "Judge" agents.
**Context**: Guarantees adversarial independence.

#### Scenario: Tool invocation with a git diff
**GIVEN** a git diff and an architectural proposal
**WHEN** `scouter_judge` is called
**THEN** the system MUST spawn two parallel sampling sessions
**AND** each session MUST use the `JudgeSystemPrompt`

### Requirement: REQ-JUDGE-2 - Cynical Scrutiny
The Judge agents MUST identify at least 3 risk vectors (Security, Performance, Logic).
**Context**: Prevents superficial reviews.

### Requirement: REQ-JUDGE-3 - Verdict Synthesis
The tool MUST synthesize a single report containing the average rating and consolidated findings.
**Context**: Unified feedback for the author agent.

#### Scenario: Divergent Ratings
**GIVEN** Judge-A gives 9.0/10 and Judge-B gives 6.0/10
**WHEN** the results are synthesized
**THEN** the system MUST mark the verdict as "CONTESTED"
**AND** it MUST flag the absolute divergence (3.0)

### Requirement: REQ-JUDGE-4 - Verdict Levels
Ratings MUST map to the following levels:
- **[9.0 - 10.0]**: SOVEREIGN (Divine Class)
- **[8.0 - 8.9]**: ACCEPTABLE (Minor Redemption required)
- **[< 8.0]**: HAKAI (Erased / Blocked)

## 3. Constraints
- **Max Tokens**: Each judge session is capped at 2048 tokens.
- **Language**: All reasoning MUST be delivered in Technical English.
