# internal/mcp — Agent Instructions

## Overview
This directory contains the Model Context Protocol (MCP) server implementation. It serves as the primary interface for AI agents to interact with the analysis engine.

## Components
- `server.go`: Manages server initialization and tool registration.
- `handlers.go`: Implements tool handlers as thin adapters to the core engine.
- `resources.go`: Manages access to read-only project resources and documentation.
- `messenger.go`: Provides an adapter for sampling and user interaction protocols.

## Development Guidelines
- **Standardized Handlers**: All tool handlers must implement the standard MCP signature, ensuring consistent request processing.
- **Analytical Reasoning**: Include a `<thought>` block at the beginning of responses to provide context on the analysis performed.
- **Engine Delegation**: Delegate all complex analytical and modification tasks to the core analysis engine (`s.engine`).
- **Data Optimization**: Filter and truncate large datasets to ensure that responses remain focused and relevant.

## Operational Boundaries
- ✅ **Required:** Register all new analysis tools in the server configuration.
- ⚠️ **Review Needed:** Consult on changes to the sampling logic or the introduction of additional transport layers.
- 🚫 **Prohibited:** Implementing business or analysis logic directly within the handler adapters.
