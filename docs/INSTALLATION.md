# Installation and Setup Guide

Scouter can be deployed as a standalone Command Line Interface (CLI) tool or integrated as a Model Context Protocol (MCP) server for use with AI assistants and automated workflows.

## Quick Installation

1. **Install via Go**:
   ```bash
   go install github.com/Rogercode97/scouter/cmd/scouter@latest
   ```
2. **Verify Installation**:
   ```bash
   scouter --help
   ```

## MCP Server Integration

To integrate Scouter with tools like the Gemini CLI or Claude Desktop, register it as an MCP server in your configuration.

### 1. Locate the Binary
Determine the absolute path to your Scouter installation:
```bash
which scouter
```

### 2. Configure the Server
Add the Scouter configuration to the `mcpServers` section of your `settings.json` (typically located in `~/.gemini/settings.json` or your application's config directory):

```json
{
  "mcpServers": {
    "scouter": {
      "command": "/path/to/your/bin/scouter",
      "args": ["mcp"],
      "trust": true
    }
  }
}
```

### 3. Grant Permissions
Scouter requires read and write access to your project files. Ensure the target directories are authorized for use by the application.

## System Requirements

| Component | Specification |
| :--- | :--- |
| **Go Runtime** | Version 1.24 or higher |
| **Database** | SQLite (internalized) |
| **Parser** | Tree-sitter (internalized) |

---
*Proper configuration ensures optimal analysis performance.*
