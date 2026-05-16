---
name: scouter-ripple-protocol
description: "WHEN: 'ripple', 'propagate', 'rename', 'interface change'. Protocol for atomic, multi-file symbol refactoring using Scouter's Ripple Engine."
version: "1.0.0"
tags: [refactoring, ripple, evolution]
last_updated: 2026-05-16
---

# Scouter Ripple Protocol (Wave 14.5)

**Goal**: Execute atomic and consistent symbol transformations across the entire project hierarchy using the Ripple Engine.

> **LIVE LIBRARY**: See `internal/engine/ripple.go` for the transformation strategies and validation logic.

## 🔱 MANDATES
- **Atomic Evolution**: Transformations MUST be staged as a single unit in the Ledger.
- **Interface Awareness**: Always propagate changes from interfaces to all their implementations.
- **Validation Before Seal**: Every ripple MUST pass the `Validator` interface before being committed.

## 🔄 PROTOCOL
1. **Identify Start**: Select the source symbol and the desired transformation.
2. **Scout Reach**: Run `scouter impact` to identify all affected call sites and implementations.
3. **Propagate**: Invoke the `ripple_refactor` tool to stage changes in the Ledger.
4. **Verify Integrity**: Run the `Validator` (e.g., `scouter predict`) to ensure the ripple didn't break the build.
5. **Seal**: Commit the Ledger changes once verified.

## 🚩 RED FLAGS
- Manually renaming symbols in multiple files without using the Ripple Engine.
- Skipping the validation phase for a complex ripple.
- Partial propagation (leaving broken references in dependent packages).

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "I'll just use search-and-replace." | Raw text replacement is not symbol-aware and often misses complex interface implementations. |
| "It's only two files, I don't need Ripple." | Even two files can have hidden dependencies. Ripple ensures zero blind spots. |

## 📜 SUCCESS HEURISTIC
The transformation is complete, all references are updated, and the project passes all validation checks without manual intervention.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: `scouter`.
<!-- MCP:END -->
