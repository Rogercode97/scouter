# SDD Design: MUNCH Density Format

**What**: Technical design for MUNCH (Multi-Path Unit Compact Hierarchy) format implementation.
**Why**: To fulfill the sdd/munch-density-format/spec by providing a high-density, token-efficient wire format for MCP responses.
**Where**: 
- `internal/display/density.go`
- `internal/display/density_test.go`
- `internal/mcp/handle_analysis.go`
- `internal/mcp/handle_intelligence.go`
**Learned**: 
- The `MUNCHEncoder` will be stateful, maintaining a `map[string]int` for path interning and writing to an `io.Writer`.
- MCP handlers will accept an optional `format` parameter. If `format == "munch"` or the result set exceeds a threshold (e.g., > 20 items), the handler will use `MUNCHEncoder` instead of JSON marshaling.
- Tags: `S` (Symbol), `C` (Call), `R` (PageRank), `K` (Churn), `X` (Critical Symbol).
- Rejected stateless encoding because path interning requires state across the result set.
