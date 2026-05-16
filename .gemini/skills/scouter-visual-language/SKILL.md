---
name: scouter-visual-language
description: "WHEN: 'styling', 'ui', 'lipgloss', 'colors'. Rules for consistent TUI visual identity using Scouter's display package."
version: "1.0.0"
tags: [ui, tui, styling]
last_updated: 2026-05-16
---

# Scouter Visual Language (Wave 14.5)

**Goal**: Maintain a consistent, high-density, and authoritative TUI aesthetic across all Scouter outputs using the `internal/display` package.

> **LIVE LIBRARY**: See `internal/display/display.go` for the source of truth on styles and colors.

## 🔱 MANDATES
- **Style Consistency**: ALWAYS use the predefined styles in `internal/display/display.go` (e.g., `HeaderStyle`, `SuccessStyle`).
- **Terminal Awareness**: Use `IsTerminal()` to conditionally apply styles. Never output raw ANSI codes in non-TTY environments.
- **High Density**: Prefer horizontal separators and compact layouts to maximize information signal.

## 🎨 PALETTE & STYLES
| Style | Color/Effect | Purpose |
|---|---|---|
| `HeaderStyle` | Bold, Color 12 | Main headings and labels. |
| `SuccessStyle` | Color 10 | Successful operations and confirmations. |
| `ErrorStyle` | Color 9 | Failure messages and critical errors. |
| `DimStyle` | Color 8 | Metadata, timestamps, and secondary info. |
| `StatStyle` | Bold, Color 14 | Key metrics and statistics. |

## 🔄 PROTOCOL
1. **Import**: Import the `display` package in your UI logic.
2. **Apply**: Wrap your text with the appropriate style (e.g., `display.HeaderStyle.Render("Target")`).
3. **Validate**: Check output in both TTY and non-TTY environments.

## 🚩 RED FLAGS
- Hardcoding hex codes or ANSI escapes outside of the `display` package.
- Outputting colored text to non-terminal environments.
- Using inconsistent spacing or separators.

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "This is just a temporary print." | Temporary code often becomes permanent. Use styles from the start. |
| "I want this to pop more with a new color." | Deviating from the palette dilutes the brand and confuses the agent. |

## 📜 SUCCESS HEURISTIC
Outputs are visually cohesive, prioritize signal over noise, and degrade gracefully in non-interactive shells.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: None.
<!-- MCP:END -->
