---
name: scouter-impact-analysis
description: "WHEN: 'impact', 'blast radius', 'risk', 'critical code'. Protocol for assessing the risk and reach of changes using Scouter's Impact Engine."
version: "1.0.0"
tags: [analysis, risk, impact]
last_updated: 2026-05-16
---

# Scouter Impact Analysis (Wave 14.5)

**Goal**: Determine the "blast radius" of any proposed change to prevent unintended side effects and protect critical code paths.

> **LIVE LIBRARY**: See `internal/engine/impact.go` for the recursive call graph traversal and risk scoring logic.

## 🔱 MANDATES
- **Analysis Before Action**: ALWAYS run `scouter impact` or use the `impact` MCP tool before making non-trivial changes.
- **Critical Code Protection**: If a change affects symbols with high centrality (hotspots), a manual review or a *Divine Trial* (Sacrifice Snippet) is MANDATORY.
- **Depth Control**: Use a minimum depth of 3 for recursive analysis to capture indirect dependencies.

## 🔄 PROTOCOL
1. **Target Identification**: Specify the symbol (function, method, struct) to be analyzed.
2. **Execute Analysis**: Run the `impact` tool to generate the blast radius map.
3. **Review Risk**: Analyze the centrality and fragility of the affected symbols.
4. **Predict Tests**: Use `scouter predict` to identify which test suites MUST pass to verify the change.
5. **Mitigate**: If the impact is too large, consider refactoring to decouple the symbol before proceeding.

## 🚩 RED FLAGS
- Proceeding with a change when the `impact` tool reports a high-risk score without mitigation.
- Ignoring indirect callers (depth > 1) in complex systems.
- Failing to identify the "Critical Path" of a feature.

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "I know where this is used." | Human memory is fallible and misses indirect dependencies or interface implementations. |
| "It's just a local change." | In a connected system, no change is truly local until proven by the Impact Engine. |

## 📜 SUCCESS HEURISTIC
The agent has a complete map of the change's reach, has identified all affected test suites, and has verified that critical hotspots are not negatively impacted.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: `scouter`.
<!-- MCP:END -->
