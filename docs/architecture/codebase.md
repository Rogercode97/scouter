# Codebase Navigation Guide

**This guide is for maintainers and contributors who need to understand where Scouter's responsibilities live, which invariants are non-negotiable, and which file to open when something needs to change.**

Scouter is a structural intelligence engine. The center of the product is a Go binary that parses ASTs and writes to SQLite; the CLI, MCP, and analysis engines are interfaces around that core.

## Quick Map: If you need X, read Y

| If you need to... | Open first | Then check |
|---|---|---|
| Understand the system design | `docs/ARCHITECTURE.md` | `README.md` |
| Change MCP tools | `internal/mcp/server.go` | `internal/mcp/handlers.go`, `internal/mcp/AGENTS.md` |
| Change core analysis logic | `internal/engine/truth.go` | `internal/engine/AGENTS.md`, `internal/engine/*_test.go` |
| Change AST parsing or storage | `internal/store/store.go` | `internal/engine/treesitter.go` |
| Change CLI output formatting | `internal/display/display.go` | `internal/display/density.go` |
| Prepare or review a large feature | `openspec/changes/*` | `CONTRIBUTING.md`, `openspec/specs/*` |

## 📂 Repository Structure

| Directory | Responsibility | Key Components |
| :--- | :--- | :--- |
| `cmd/scouter` | Entry point for the CLI and MCP Server. | CLI commands, server initialization. |
| `internal/mcp` | Model Context Protocol (MCP) server implementation. | Tool handlers, resource providers. |
| `internal/engine` | Core analysis logic and processing engines. | Search, Impact, Refactoring, and Diagnostics. |
| `internal/store` | Persistence layer and data management. | SQLite integration, indexing logic. |
| `internal/adapters`| External service integrations and adapters. | Memory and third-party service providers. |
| `internal/display` | Output formatting and user interface logic. | CLI rendering, data serialization. |
| `openspec` | Specification-Driven Development (SDD) documentation. | Project specifications and task tracking. |

## 🚀 Key Data Flows

### 1. Initialization
The application initializes in `cmd/scouter/main.go`. It sets up the storage layer, configures the analysis engines, and starts the requested interface (either the interactive CLI or the MCP server).

### 2. Analysis Request Pipeline
1. **Interface Layer** (`internal/mcp`): Receives and validates incoming requests.
2. **Orchestration Layer** (`internal/engine`): Coordinates the necessary analysis tasks.
3. **Execution Engines**: Specific tasks are delegated to specialized engines (e.g., impact analysis in `internal/engine/impact.go`).
4. **Storage Access**: Data is retrieved from the indexed symbols or the call graph in the storage layer.

### 3. Change Management Workflow
1. **Proposal**: A code modification is proposed through the refactoring engine.
2. **Impact Assessment**: The system calculates the dependency chain to identify potential side effects.
3. **Staging**: Changes are recorded in a staging ledger for validation.
4. **Application**: Once verified, changes are committed to the filesystem.

## 🛠️ Development Guidelines

- **Architectural Integrity**: Maintain a clear separation between domain logic (`internal/engine`) and interface adapters (`internal/mcp`).
- **Testing Requirements**: All core analysis features must include comprehensive unit and integration tests.
- **Data Density**: Ensure that analysis results are concise and relevant, avoiding excessive output in automated environments.

---
*Structural clarity is the foundation of maintainable software.*
