# Specification: Go 1.25 Optimizations

## NEW Requirements

### Requirement: Resource Safety
Scouter MUST use `runtime.AddCleanup` (Go 1.24+) for DB and LSP Manager instances to ensure reliable cleanup.

**Scenario:** Database instance cleanup
- **GIVEN** a Database instance is created
- **WHEN** the Database instance is no longer reachable and gets garbage collected
- **THEN** the system MUST execute the `runtime.AddCleanup` callback to close the connection properly

**Scenario:** LSP Manager instance cleanup
- **GIVEN** an LSP Manager instance is created
- **WHEN** the LSP Manager instance is no longer reachable and gets garbage collected
- **THEN** the system MUST execute the `runtime.AddCleanup` callback to terminate the LSP process properly

### Requirement: Performance
Scouter SHOULD use `encoding/json/v2` for heavy JSON operations (Delta Export/Import and Engram integration) to reduce latency.

**Scenario:** Delta Export execution
- **GIVEN** the system needs to export a large delta payload
- **WHEN** serializing the payload to JSON
- **THEN** the system SHOULD use `encoding/json/v2` to achieve lower latency

**Scenario:** Engram integration processing
- **GIVEN** the system needs to parse a heavy JSON payload from the Engram
- **WHEN** deserializing the payload
- **THEN** the system SHOULD use `encoding/json/v2` to achieve lower latency

### Requirement: AST Efficiency
For Go files, Scouter MAY use native `go/ast` with `PreorderStack` if it outperforms Tree-sitter for symbol extraction.

**Scenario:** Symbol extraction on a Go file
- **GIVEN** the system needs to extract symbols from a Go source file
- **WHEN** performing the AST extraction
- **THEN** the system MAY use native `go/ast` with `PreorderStack` instead of Tree-sitter, provided it yields better performance
