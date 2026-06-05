# Agent Setup: Scouter 🕶️

Scouter is designed to be the "eyes" for your AI coding agents, providing deep structural intelligence (AST) that standard tools (grep, cat) miss.

## 📡 The MCP Protocol

Scouter uses the **Model Context Protocol (MCP)** to communicate with agents. When connected, the agent gains access to the **Sovereign Arsenal**.

### 🛠️ The Arsenal (27 Tools)

Scouter tools are optimized for structural reasoning. They are divided into **Core** and **Specialized** (Heavy Arsenal).

#### Core Tools (Always Available)
| Tool | Purpose |
| :--- | :--- |
| `index` | Maps a directory to the AST store. |
| `search` | Semantic and text search for code symbols. |
| `read` | Reads specific symbol fragments with high precision. |
| `callers` | Identifies all callers of a function/method. |
| `goto_definition` | LSP-based jump to definition. |
| `type_info` | Retrieves type documentation and hover info. |
| `impact` | Calculates the "Blast Radius" of a change. |
| `scouter_commit` | Persists staged changes from the Ledger. |
| `scouter_rollback` | Discards staged changes. |
| `scouter_diff` | Previews staged changes. |

#### Heavy Arsenal (Unlock via `unlock_heavy_arsenal`)
| Tool | Purpose |
| :--- | :--- |
| `self_heal` | Autonomous RCA -> Fix -> Verify loop. |
| `ripple_refactor` | Project-wide symbol propagation. |
| `critical_code` | Identifies high-risk/fragile symbols. |
| `structural_search` | Search via AST patterns (ast-grep style). |
| `evolve` | Apply multi-file architectural changes safely. |
| `predict` | Identify affected tests from local changes. |
| `scouter_judge` | Adversarial review of proposals. |

---

## 🚀 Setup Guides

Scouter provides automated MCP integration for 7 popular agents and IDEs via the `setup` command.

### Automated Integrations

Run `scouter setup <agent>` to automatically inject the MCP configuration into the respective agent's settings:

- **Gemini CLI**: `scouter setup gemini-cli`
- **Antigravity CLI**: `scouter setup antigravity-cli`
- **OpenCode**: `scouter setup opencode`
- **Codex**: `scouter setup codex`
- **Cursor**: `scouter setup cursor` (modifies project-relative `.cursor/mcp.json`)
- **Windsurf**: `scouter setup windsurf`

### Claude Code
Install Scouter as a plugin using the absolute path to the binary:
```bash
claude plugin install $(which scouter)
```
*(Alternatively, use `scouter setup claude` to inject a token-killer hook into Claude's bash execution pipeline).*

### VS Code (Manual Setup)
Add Scouter as an MCP server in your IDE settings.

**For VS Code (using the MCP extension):**
Add this to your MCP configuration JSON:
```json
{
  "mcpServers": {
    "scouter": {
      "command": "scouter",
      "args": ["mcp"]
    }
  }
}
```
---

## 🧠 Memory Synergy (Scouter + Engram)

While **Engram** provides the **Brain** (Memory), **Scouter** provides the **Eyes** (Analysis). 

For the best experience, use Scouter tools to identify *where* to change code, and Engram tools (`mem_save`, `mem_search`) to remember *why* those changes were made. Scouter can also save summaries directly to Engram via `save_anchor`.
