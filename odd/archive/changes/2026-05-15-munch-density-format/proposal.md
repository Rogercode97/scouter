# SDD Proposal: MUNCH Density Format

**What**: Implement MUNCH (Multi-Path Unit Compact Hierarchy), a high-density wire format for symbol and file data in MCP responses.
**Why**: Large symbol maps and search results currently consume excessive tokens (Ki), limiting the effectiveness of LLM analysis in large codebases. MUNCH reduces overhead via path interning and tabular packing.
**Where**: internal/display/density.go, internal/mcp/handle_analysis.go, internal/mcp/handle_intelligence.go
**Learned**: Inspired by jCodeMunch, this format prioritizes machine-readability and token efficiency over human-readable aesthetics for high-volume data transfers.

**Scope**:
- **In-Scope**:
  - `internal/display/density.go`: MUNCH encoder implementing Path Interning and Tabular Packing.
  - MCP Integration: Updating `handle_analysis` (search) and `handle_intelligence` (PageRank/Churn) to support MUNCH encoding.
  - `internal/display/density_test.go`: Comprehensive round-trip and token-efficiency benchmarks.
- **Out-of-Scope**:
  - Modification of core AST indexing logic.
  - Changes to default CLI terminal output (MUNCH is for wire/LLM context).

**Approach**:
- Header: `#MUNCH/1` for versioning and detection.
- Legend Section: Map common file paths to short IDs (e.g., `@1:internal/engine/analyzer.go`).
- Data Section: Compact rows using Legend IDs (e.g., `1|SymName|Function|12|45`).
- Rationale: Deterministic packing ensures the LLM sees consistent patterns, improving attention mechanism efficiency.
