# Scouter 🕶️

**A Structural Analysis and Intelligence Engine for Codebases.**

Scouter provides deep AST inspection, impact analysis, and automated refactoring capabilities. By indexing codebases into a queryable structure, it enables precise navigation and data-driven insights for complex software projects.

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

## 🛠️ Tool Reference

| Tool | Purpose |
| :--- | :--- |
| `diagnose` | Performs root cause analysis and validates potential fixes. |
| `refactor` | Propagates changes across interfaces and implementations. |
| `impact` | Analyzes the blast radius of a symbol change. |
| `critical_path` | Identifies central symbols and potential points of failure. |
| `search` | Pattern-based discovery using AST structures. |

## 📚 Documentation

- [Architecture Overview](./docs/ARCHITECTURE.md)
- [Codebase Guide](./docs/CODEBASE-GUIDE.md)
- [Installation & MCP Setup](./docs/INSTALLATION.md)
- [Security Policy](./SECURITY.md)

---
*MIT License*
