# Scouter — Agent Instructions

## Overview
Scouter is a sovereign code analysis engine and Architectural Oracle designed for AI agents. It provides deep AST inspection, impact analysis, and self-healing capabilities via the Model Context Protocol (MCP).

## Build & Run
- `just build`          # Build the production binary in bin/
- `make build`          # Alternative build command
- `just run`           # Run the analyzer from source
- `go run ./cmd/scouter` # Direct run command

## Testing
- `just test`          # Run all tests
- `go test ./...`      # Standard Go test command
- `go test -v ./internal/mcp/...` # Test MCP handlers specifically

## Project Structure
- `cmd/scouter/`       # Main entry point and CLI plugins
- `internal/mcp/`      # MCP Server implementation (Wave 12.0)
- `internal/engine/`   # TruthEngine, Analysis, and Ripple Logic
- `internal/store/`    # Persistence layer (SQLite/Engram)
- `openspec/`          # Specifications and Change records
- `tests/`             # High-fidelity integration tests

## Code Style & Conventions
- **Hexagonal Architecture**: Keep domain logic (engine) isolated from adapters (mcp, store).
- **Context Sovereignty**: Always pass and respect `context.Context`.
- **Pure Signal**: Minimize output slop. Use `<thought>` blocks in MCP responses.
- **Go 1.25+**: Use modern Go patterns (iterators, structured logging).

### Example
```go
func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	// 1. Analyze (Thought)
	thought := fmt.Sprintf("<thought>\nSearching for '%s'...\n</thought>\n", args.Query)
	
	// 2. Execute
	results, err := s.store.SearchSymbols(ctx, args.Query, args.Type, 50, 0)
	if err != nil {
		return nil, nil, err
	}
	
	// 3. Return Pure Signal
	out, _ := json.Marshal(results)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: thought + string(out)}},
	}, nil, nil
}
```

## Boundaries
- ✅ **Always do:** Run `just build` and `just test` before finalizing any change.
- ⚠️ **Ask first:** Modifying core `TruthEngine` logic or breaking the MCP protocol schema.
- 🚫 **Never do:** Commit secrets, API keys, or large binary artifacts. Do not use legacy SDKs.

## Documentation
- ADRs: `docs/adr/` — Architecture Decision Records (Access via MCP Resources).
- Specs: `openspec/specs/` — Formal project specifications.
