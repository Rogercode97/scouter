# 🚀 Scouter Installation & Setup

Scouter can be used as a standalone CLI tool or as a Model Context Protocol (MCP) server to empower your AI agents.

## Quick Start

1. **Install Binary**:
   ```bash
   go install github.com/Rogercode97/scouter/cmd/scouter@latest
   ```
2. **Verify**:
   ```bash
   scouter --help
   ```

## MCP Server Registration

To use Scouter with the Gemini CLI or Claude Desktop, you must register it in your `settings.json`.

### 1. Identify Binary Path
Find where your scouter binary is located:
```bash
which scouter
```

### 2. Update Configuration
Add the following to your `mcpServers` object in `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "scouter": {
      "command": "/ruta/a/tu/bin/scouter",
      "args": ["mcp"],
      "trust": true
    }
  }
}
```

### 3. Folder Trust
Scouter requires access to your source code. Ensure the project folder is trusted by running:
```bash
gemini trust
```

## Advanced Setup

| Requirement | Details |
| :--- | :--- |
| **Go Version** | 1.24+ (Uses `runtime.AddCleanup`) |
| **SQLite** | Required for the persistence vault (included in binary) |
| **Tree-sitter** | Used for AST parsing (included in binary) |

---
*The system is only as sovereign as its installation. Hakai.*
