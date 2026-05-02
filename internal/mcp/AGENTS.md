# internal/mcp — Agent Instructions

## Overview
This folder contains the Model Context Protocol (MCP) server implementation. It acts as the primary adapter for AI agents to interact with Scouter's TruthEngine.

## Structure
- `server.go`    # Server initialization and tool registration.
- `handlers.go`  # Implementation of tool handlers (Logic-agnostic).
- `resources.go` # Registration of read-only resources (ADRs, Workspace).
- `messenger.go` # Adapter for MCP Sampling (Ask user/model).

## Development Guidelines
- **Tool Handlers**: All handlers must follow the signature `func (s *Server) handleX(ctx context.Context, req *mcp.CallToolRequest, args XParams) (*mcp.CallToolResult, any, error)`.
- **Reasoning**: Always include a `<thought>` block at the beginning of the `TextContent`.
- **TruthEngine**: Delegate all complex analytical or mutation logic to `s.engine` (TruthEngine).
- **Pure Signal**: Truncate large results and filter for the "Truth Kernel".

## Boundaries
- ✅ **Always do:** Register new tools in `server.go`.
- ⚠️ **Ask first:** Adding new transport layers or changing the sampling logic.
- 🚫 **Never do:** Write business logic directly in `handlers.go`. Keep them as thin adapters.
