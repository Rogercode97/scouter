# Scouter 🕶️

**A Structural Analysis and Intelligence Engine for Codebases.**

Scouter provides deep AST inspection, impact analysis, and automated refactoring capabilities. By indexing codebases into a queryable structure, it enables precise navigation and data-driven insights for complex software projects.

## 90-Second Mental Model

```text
AI Agent (Claude / Gemini / Cursor)
        │
        │ Requests analysis, impact assessment, or refactoring
        ▼
cmd/scouter (MCP Server & CLI)
        │
        ├── internal/mcp        Tools: search, impact, refactor, diagnose
        │
        ▼
internal/engine (Analysis Engine)
  Orchestrates structural analysis and safe mutation
        │
        ├── impact.go           Calculates the "Blast Radius"
        ├── ripple.go           Propagates changes across the AST
        ├── healer.go           Diagnoses and verifies fixes
        │
        ▼
internal/store (Persistence & Indexing)
  SQLite + Tree-sitter (The structural map of the codebase)
```

> **Core Invariant:** Validation is finality. No destructive mutation is applied without passing through the Staging Ledger and Impact Analysis.

## 🚀 Quick Start

1. **Install the binary:**
   ```bash
   go install github.com/Rogercode97/scouter/cmd/scouter@latest
   ```
2. **Verify installation:**
   ```bash
   scouter --version
   ```
3. **Run a structural search:**
   ```bash
   scouter search "func main"
   ```

## 🛠️ Key Capabilities

| Capability | Description | Benefit |
| :--- | :--- | :--- |
| **Stateful Context Management** | Adaptive context windowing for LLM integrations. | Significant reduction in token consumption. |
| **High-Density Data Format** | Efficient serialization using path interning. | Handles large result sets without context overflow. |
| **Persistence Layer Integration** | Connects AST data with historical metadata. | Tracks long-term changes and refactoring history. |
| **Impact Analysis** | Calculates the reach of changes across the codebase. | Identifies potential regressions and dependencies. |
| **Automated Diagnostics** | Root cause analysis and automated verification loops. | Accelerates debugging and code maintenance. |

## 🛡️ Security & Reliability

- **Staging Ledger**: All destructive operations are staged and validated before being committed to disk.
- **Result Truncation**: Automated signal-to-noise filtering to maintain focus on relevant data.
- **Security Auditing**: Regularly scanned with `gosec`, `govulncheck`, and CodeQL to ensure robustness.

## 📚 Documentation Map

| If you need to... | Read this |
| :--- | :--- |
| Understand the system design | [Architecture Overview](./docs/ARCHITECTURE.md) |
| Find where logic lives | [Codebase Guide](./docs/CODEBASE-GUIDE.md) |
| Connect an AI agent | [Installation & MCP Setup](./docs/INSTALLATION.md) |
| Report a vulnerability | [Security Policy](./SECURITY.md) |

---
*MIT License*
