# Scouter Architecture

Scouter is a structural intelligence engine designed to facilitate deep codebase analysis and automated code manipulation. It combines static analysis, call graph generation, and impact assessment into a unified system accessible via CLI or the Model Context Protocol (MCP).

## Design Principles

| Principle | Description |
| :--- | :--- |
| **Hexagonal Architecture** | The core logic (analysis engines) is isolated from external interfaces (MCP, CLI) via defined ports and adapters. |
| **Explicit Organization** | Directory structures (`internal/engine`, `internal/mcp`, `internal/store`) clearly separate analysis, communication, and storage concerns. |
| **Verified Mutation** | All code modifications pass through a staging ledger and impact analysis before execution to ensure system integrity. |
| **Separation of Concerns** | Specialized engines handle specific tasks (parsing, search, impact, refactoring) to maintain a maintainable and scalable codebase. |

## System Components

### 1. Analysis Engines (`internal/engine`)
The core functionality of Scouter, responsible for structural understanding:
- **Indexer Pipeline**: Utilizes a **Worker Pool + Single Collector** pattern for high-performance indexing, enabling concurrent AST parsing while maintaining atomic database writes via a dedicated collector.
- **Semantic Engine**: Resolves types and builds the structural model.
- **Search Engine**: AST-aware pattern matching using Tree-sitter.
- **Impact Engine**: Calculates dependency chains and change propagation through the global call graph.
- **Refactoring Engine**: Handles coordinated changes across multiple files and interfaces.
- **Diagnostic Engine**: Provides automated root cause analysis and verification of fixes.

### 2. Data Persistence (`internal/store`)
A persistent storage layer backed by SQLite that manages:
- **Symbol Registry**: An indexed database of AST nodes and their associated metadata.
- **Call Graph Database**: A map of functional relationships and dependencies.
- **Contextual Memory**: Integration with persistent storage to maintain analysis history across sessions.

### 3. Integration Adapters (`internal/mcp`)
Interfaces for external consumers, primarily AI agents:
- **MCP Implementation**: Provides tool definitions and resource access for LLM environments.
- **Resource Management**: Facilitates access to analysis reports, staging ledgers, and project metadata.
- **System Instructions**: Standardized operational protocols for integrated tools.

## Standard Workflow

1. **Discovery**: Index the project and identify structural patterns using search tools.
2. **Analysis**: Assess the impact and risks associated with proposed changes.
3. **Staging**: Prepare atomic modifications in the staging ledger for review.
4. **Verification**: Validate the integrity of changes using diagnostic tools and tests.
5. **Execution**: Commit verified changes to the file system.

---
*Engineering Excellence through Structural Integrity.*
