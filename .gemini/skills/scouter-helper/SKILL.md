---
name: scouter-helper
description: "WHEN: 'scouter', 'commands', 'how to use'. Reference for all available Scouter CLI commands and their purposes."
version: "1.0.0"
tags: [cli, reference, help]
last_updated: 2026-05-16
---

# Scouter Helper (Wave 14.5)

**Goal**: Provide a quick and reliable reference for Scouter CLI commands, ensuring agents use the right tool for the job.

> **LIVE LIBRARY**: See `internal/cli/cli.go` for the command dispatcher logic.

## 🔱 MANDATES
- **Command Awareness**: Always check if a specialized Scouter command (e.g., `predict`, `index`) is more efficient than raw shell commands.
- **Filtering by Default**: Scouter automatically filters the output of proxied commands (like `go test`) to minimize token usage.
- **Shadow Indexing**: Almost all commands trigger background "Shadow Indexing" to keep the AST truth updated.

## 🛠️ BUILT-IN COMMANDS
| Command | Purpose | Usage Example |
|---|---|---|
| `mcp` | Starts the Model Context Protocol (MCP) server. | `scouter mcp` |
| `index` | Manually indexes a file or directory into the AST store. | `scouter index ./src` |
| `gain` | Shows token savings statistics and adoption history. | `scouter gain` |
| `setup` | Runs the initial configuration and environment check. | `scouter setup` |
| `predict` | Predicts affected tests based on current git diff. | `scouter predict` |

## 🔄 PIPELINE COMMANDS (Proxied)
Scouter acts as a wrapper for common tools, injecting its filtering pipeline:
- `scouter go test`: Runs tests and shows only failures (ingests results for fragility mapping).
- `scouter git diff`: Shows an ultra-condensed diff.
- `scouter any-command`: If no filter matches, it passes through and performs "Shadow Indexing" in the background.

## 🔄 PROTOCOL
1. **Identify Task**: Determine if the task involves indexing, test prediction, or token savings analysis.
2. **Select Command**: Use the corresponding Scouter command (e.g., `scouter predict`).
3. **Execute**: Run the command and analyze the filtered output.

## 🚩 RED FLAGS
- Using raw commands (e.g., `go test`) when `scouter go test` would provide better signal and update the health map.
- Manually indexing files that Scouter would automatically "Shadow Index".

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "I'll just run go test to be sure." | `scouter go test` runs the same tests but captures results for Scouter's intelligence map. |
| "I don't need to index, I can just read files." | Indexing enables `impact` analysis and `structural_search`, which are far more powerful than raw reads. |

## 📜 SUCCESS HEURISTIC
The agent uses the most specific Scouter command available, maximizing context density and maintaining the project's analytical integrity.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: None.
<!-- MCP:END -->
