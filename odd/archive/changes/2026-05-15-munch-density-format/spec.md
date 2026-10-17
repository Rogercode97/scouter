# MUNCH Density Format Specification

## Domain: MUNCH (Multi-Path Unit Compact Hierarchy)

### Requirement: Versioned Protocol Header
The MUNCH encoder MUST prepend a versioned header to all encoded streams to ensure client-side detection and future protocol compatibility.
- GIVEN a set of symbol or file results
- WHEN the MUNCH encoder is invoked
- THEN the first line of the output MUST be exactly `#MUNCH/1`.

### Requirement: Path Interning (Legend Section)
The format MUST intern recurring file paths into short handles to minimize token redundancy in large codebases.
- GIVEN a result set with multiple symbols in `internal/engine/analyzer.go`
- WHEN encoded via MUNCH
- THEN the output MUST include a legend section mapping a unique ID to the path (e.g., `@1:internal/engine/analyzer.go`)
- AND data rows MUST use the ID handle (e.g., `1`) instead of the full path string.

### Requirement: Tagged Tabular Rows
Data MUST be packed into compact, pipe-separated (`|`) rows prefixed by a single-character type tag to facilitate high-density parsing.
- GIVEN a symbol search result
- WHEN encoded
- THEN it MUST appear as a row starting with `S` (Symbol) followed by interned path ID, name, kind, line, and character.
- AND a caller relationship MUST appear as a row starting with `C` (Call) followed by caller path ID and name.

### Requirement: Deterministic Streaming
The encoder MUST be deterministic and use O(1) memory relative to result size by streaming output.
- GIVEN a result set of any size
- WHEN encoding
- THEN the encoder SHALL emit rows incrementally as it iterates the result set.
- AND identical input sets MUST produce byte-identical MUNCH output.

## Domain: MCP Tooling Integration

### Requirement: Format Selection Argument
Supported MCP tools MUST accept an optional `format` argument to toggle between high-density and human-readable output.
- GIVEN a request to `handleSearch`, `handleCallers`, or `handleIntelligence`
- WHEN the `format` argument is set to `"munch"`
- THEN the tool response MUST be encoded in the MUNCH format.
- AND if `format` is omitted or set to `"text"`, the legacy human-readable output MUST be returned.

### Requirement: Intelligence Data Tags
The MUNCH format MUST provide specific tags for intelligence metrics (PageRank, Churn).
- GIVEN a PageRank or Churn report request with `format: "munch"`
- WHEN encoding the results
- THEN it MUST use the `R` (Rank) tag for PageRank scores and `K` (K-Churn) tag for churn metrics.
- AND rows MUST include the interned path ID followed by the metric value (e.g., `R|1|0.85`).
