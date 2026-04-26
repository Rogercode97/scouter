# Impact Analysis Specification (Omniscience Edition)

## Purpose
The Impact Analysis engine provides a recursive traversal of the call graph to identify entities potentially affected by a change. It calculates a **Predictive Risk Score** and generates a **Visual Blast Radius Map** to surface critical architectural risks.

## Requirements

### Requirement: Recursive Entity-Level Traversal
The system MUST be able to traverse the call graph upwards (caller <- callee) recursively at the entity level (functions/methods).
(Previously: Recursive Caller Traversal)

#### Scenario: Recursive Callers Discovery
- GIVEN a call graph where `FuncA` -> `FuncB` -> `FuncC`
- WHEN `scouter_impact` is called for `FuncC` with `maxDepth` >= 2
- THEN the result MUST include `FuncB` at distance 1 and `FuncA` at distance 2.

### Requirement: Predictive Risk Scoring
The system MUST calculate a Risk Score between 0.0 and 1.0 for the target entity and its blast radius.
(Previously: New capability)

#### Scenario: High Centrality Risk
- GIVEN a function `Initialize` that is called by 20 different modules
- WHEN `scouter_impact` is called for `Initialize`
- THEN the `risk_score` MUST be >= 0.8
- AND the `risk_level` MUST be "Critical".

#### Scenario: Low Impact Change
- GIVEN an internal helper function `formatDate` called by only one local function
- WHEN `scouter_impact` is called for `formatDate`
- THEN the `risk_score` MUST be <= 0.3
- AND the `risk_level` MUST be "Low".

### Requirement: Visual Impact Mapping (Mermaid)
The system MUST generate a valid Mermaid.js graph string representing the impact chain.
(Previously: New capability)

#### Scenario: Mermaid Graph Generation
- GIVEN a chain `A` -> `B` -> `Target`
- WHEN `scouter_impact` is called for `Target`
- THEN the response MUST include a `mermaid` field
- AND the field MUST contain `graph TD`
- AND the field MUST contain `B[...] --> Target[...]` and `A[...] --> B[...]`.

### Requirement: Precise B-tree Disambiguation
The system MUST uniquely identify entities using B-tree indexed name and path fields.
(Previously: Symbol Disambiguation)

#### Scenario: Precise Disambiguation
- GIVEN two functions named `Update` defined in `file_a.go` and `file_b.go`
- WHEN `scouter_impact` is called with `symbolName="Update"` and `filePath="file_a.go"`
- THEN the system MUST trace callers ONLY for the `Update` entity in `file_a.go`.

### Requirement: Traversal Constraints
The system MUST enforce limits on depth (max 10) to prevent context bloat.
(Previously: Traversal Constraints)

### Requirement: MCP Interface and Output
The system SHALL return a standardized JSON structure including `risk_score`, `risk_level`, and `mermaid`.
(Previously: MCP Interface and Output)

#### Scenario: Standardized Response
- WHEN `scouter_impact` is successful
- THEN the response MUST be a JSON object containing:
  - `target`: Object with `symbol`, `file`, `risk_score`.
  - `callers`: Array of objects with `symbol`, `file`, `distance`, `risk_score`.
  - `mermaid`: String with Mermaid.js code.
  - `risk_level`: String ("Low", "Medium", "High", "Critical").

### Requirement: Recursive CTE Contract
The data store implementation MUST use a Recursive Common Table Expression (CTE) with `UNION` for deduplication and cycle immunity.
(Previously: Recursive CTE Contract)
