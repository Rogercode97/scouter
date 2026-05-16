---
name: scouter-autonomous-healing
description: "WHEN: 'fix test', 'heal', 'shinigami', 'rca'. Protocol for autonomous Root Cause Analysis and healing using Scouter's Shinigami engine."
version: "1.0.0"
tags: [workflow, healing, shinigami]
last_updated: 2026-05-16
---

# Scouter Autonomous Healing (Wave 14.5)

**Goal**: Execute the Shinigami Protocol (Solver-Verifier) to autonomously diagnose, fix, and verify test failures using Scouter's Healer Engine.

> **LIVE LIBRARY**: See `references/SHINIGAMI.md` for the deep RCA methodology and parallel solver mechanics.

## 🔱 MANDATES
- **Empirical First**: Never guess. Always start with a failing test log.
- **Ledger Verification**: All fixes MUST be staged in the Ledger and verified before committing.
- **Blast Radius**: Always calculate the impact of a proposed fix before applying it.

## 🔄 PROTOCOL
1. **Diagnose**: Run the failing test and capture the output.
2. **Heal**: Invoke the `self_heal` MCP tool (or `scouter heal`) with the error log.
3. **Verify**: The Healer Engine will automatically stage the fix in the Ledger and run the verifier.
4. **Commit**: If the verifier passes, commit the Ledger changes to disk.

## 🚩 RED FLAGS
- Attempting to manually edit files without using the Healer Engine for complex bugs.
- Skipping the Ledger verification step.
- Ignoring the blast radius of a fix.

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "I can just fix this one line manually." | Manual fixes bypass the Shinigami verification loop and risk regressions. |
| "The test passed, I don't need to check impact." | A passing test doesn't mean the architecture wasn't compromised. |

## 📜 SUCCESS HEURISTIC
Success is a verified fix committed from the Ledger, with a clear RCA trace and zero unintended blast radius.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: `scouter`.
- Use `scouter` for the `self_heal` tool and Ledger operations.
<!-- MCP:END -->
