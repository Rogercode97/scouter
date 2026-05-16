---
name: scouter-docs-alignment
description: "WHEN: 'update docs', 'code change', 'sync'. Mandatory protocol to ensure documentation (CONTEXT.md, SABIDURIA.md) matches Scouter's code truth."
version: "1.0.0"
tags: [docs, alignment, truth]
last_updated: 2026-05-16
---

# Scouter Docs Alignment (Wave 14.5)

**Goal**: Eliminate "Documentation Debt" by ensuring that Scouter's technical manuals (docs/CONTEXT.md, docs/SABIDURIA.md, README.md) are always in sync with the current state of the TruthEngine.

> **LIVE LIBRARY**: See `references/ALIGNMENT_CHECKLIST.md` for the step-by-step verification protocol.

## 🔱 MANDATES
- **Code is Truth**: Documentation MUST reflect what the code *does*, not what it *should* do.
- **Simultaneous Update**: Every behavioral code change MUST include a documentation update in the same work unit.
- **Example Validation**: All command examples in docs MUST be verified by actual execution before being committed.
- **Stale Reference Purge**: Remove or update all mentions of deprecated commands, files, or engines.

## 🔄 PROTOCOL
1. **Detect Change**: Identify if a code change affects a public interface, command, or architectural principle.
2. **Locate Impact**: Find all mentions of the changed logic in `docs/CONTEXT.md`, `docs/SABIDURIA.md`, and `docs/`.
3. **Synchronize**: Update the text and command examples to match the new reality.
4. **Verify**: Use the `scouter-helper` skill to run documented examples and prove they work.

## 🚩 RED FLAGS
- "Will update docs later" comments or mental notes.
- Documented commands that fail or return unexpected results.
- Contradictions between `CONTEXT.md` and the actual package structure.

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "The code is self-documenting." | Code shows *how*, docs show *why* and *what*. Both are needed for sovereign agents. |
| "It's a minor change, doesn't need a doc update." | Minor desyncs accumulate into total documentation failure. |

## 📜 SUCCESS HEURISTIC
A new agent can clone the repo and every single command/concept in the documentation works exactly as described on the first try.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: `scouter`.
- Use `scouter` to find all usages of a term across the documentation.
<!-- MCP:END -->
