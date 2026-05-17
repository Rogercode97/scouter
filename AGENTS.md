# Agent Instructions

## Overview
Scouter is a code analysis and intelligence engine designed for integration with AI agents. It provides AST-based code exploration, impact analysis, and automated diagnostics via the Model Context Protocol (MCP).

## Build & Execution
- `just build`          # Compiles the production binary to bin/
- `make build`          # Alternative compilation command
- `just run`           # Executes the analyzer from the source
- `go run ./cmd/scouter` # Direct execution command

## Testing Procedures
- `just test`          # Executes the complete test suite
- `go test ./...`      # Standard Go testing command
- `go test -v ./internal/mcp/...` # Verbose testing of MCP handlers

## Project Organization
- `cmd/scouter/`       # CLI entry point and server logic.
- `internal/mcp/`      # MCP server implementation and tool handlers.
- `internal/engine/`   # Core analysis engines (Search, Impact, Refactoring).
- `internal/store/`    # Data persistence and indexing layer.
- `openspec/`          # Project specifications and change tracking.
- `tests/`             # Integration and end-to-end tests.

## Development Standards
- **Modular Architecture**: Maintain clear boundaries between analysis engines and interface adapters.
- **Context Management**: Ensure all operations propagate and respect `context.Context`.
- **Informative Output**: Provide clear, concise feedback. Use `<thought>` blocks for complex analysis in MCP responses.
- **Modern Go Patterns**: Utilize Go 1.25+ features including structured logging and optimized iterators.

### Implementation Example
```go
func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	// 1. Analysis (Internal Thought)
	thought := fmt.Sprintf("<thought>\nExecuting structural search for '%s'...\n</thought>\n", args.Query)
	
	// 2. Execution
	results, err := s.store.SearchSymbols(ctx, args.Query, args.Type, 50, 0)
	if err != nil {
		return nil, nil, err
	}
	
	// 3. Response Generation
	out, _ := json.Marshal(results)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: thought + string(out)}},
	}, nil, nil
}
```

## Operational Constraints
- ✅ **Required:** Run the full test suite (`just test`) before submitting code changes.
- ⚠️ **Review Needed:** Consult on modifications to core analysis logic or protocol schemas.
- 🚫 **Prohibited:** Storing sensitive credentials or large binary files in the repository.

## Resources
- Architecture Records: `docs/adr/`
- Technical Specifications: `openspec/specs/`