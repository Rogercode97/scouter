# Specification: Ripple Engine

## Requirements

### Requirement: Path Sanitization
The system MUST sanitize and validate all file paths before performing structural analysis or tree-sitter operations. Paths MUST be constrained within the project root.

#### Scenario: Block Out-of-Bounds Path
- GIVEN a path containing traversal elements (e.g., `../../etc/passwd`)
- WHEN processed by the structural engine (`structural.go` or `treesitter.go`)
- THEN the path MUST be cleaned and validated against the project boundary
- AND it MUST return an error if it escapes the root.

### Requirement: AST Rule Integration Stability
The AST rule engine MUST handle out-of-bounds indices and empty match sets gracefully without panicking.

#### Scenario: Safe Index Access
- GIVEN an AST rule match result
- WHEN accessing capture groups or indices
- THEN the code MUST verify bounds before access
- AND `TestASTRuleIntegration` MUST pass without panicking.
