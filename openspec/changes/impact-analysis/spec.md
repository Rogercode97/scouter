# Impact Analysis Specification

## Purpose
The Impact Analysis engine provides a recursive traversal of the call graph to identify symbols potentially affected by a change. It traces callers upwards from a target symbol (callee) to its indirect callers.

## Requirements

### Requirement: Recursive Caller Traversal
The system MUST be able to traverse the call graph upwards (caller <- callee) recursively using a depth-first or breadth-first approach.
(Previously: New capability)

#### Scenario: Immediate Callers Discovery
- GIVEN a call graph where `FuncA` calls `FuncB`
- WHEN `scouter_impact` is called for `FuncB`
- THEN the result MUST include `FuncA` with a distance of 1.

#### Scenario: Recursive Callers Discovery
- GIVEN a call graph where `FuncA` -> `FuncB` -> `FuncC` (where -> means "calls")
- WHEN `scouter_impact` is called for `FuncC` with `maxDepth` >= 2
- THEN the result MUST include `FuncB` at distance 1
- AND the result MUST include `FuncA` at distance 2.

#### Scenario: Circular Dependency Handling
- GIVEN a call graph with a cycle: `FuncA` -> `FuncB` -> `FuncA`
- WHEN `scouter_impact` is called for `FuncA`
- THEN the result MUST include `FuncB` at distance 1
- AND the system MUST NOT enter an infinite loop
- AND the result set MUST contain unique symbol/file pairs for each distance level.

### Requirement: Symbol Disambiguation
The system MUST uniquely identify the target symbol using both its name and its definition path.
(Previously: New capability)

#### Scenario: Ambiguous Symbol Name
- GIVEN two functions named `Update` defined in `file_a.go` and `file_b.go`
- WHEN `scouter_impact` is called with `symbolName="Update"` and NO `filePath`
- THEN the system MUST return a `MultipleDefinitionsError`
- AND the error MUST list the available paths: `file_a.go`, `file_b.go`.

#### Scenario: Precise Disambiguation
- GIVEN two functions named `Update` defined in `file_a.go` and `file_b.go`
- WHEN `scouter_impact` is called with `symbolName="Update"` and `filePath="file_a.go"`
- THEN the system MUST trace callers ONLY for the `Update` function defined in `file_a.go`.

### Requirement: Traversal Constraints
The system MUST enforce limits on the depth of the recursive traversal to prevent performance degradation and context bloat.
(Previously: New capability)

#### Scenario: Default Depth Limit
- GIVEN a deep call chain of 5 levels
- WHEN `scouter_impact` is called WITHOUT a `maxDepth` parameter
- THEN the system MUST default to a depth of 3
- AND the result set MUST NOT include callers beyond distance 3.

#### Scenario: Maximum Depth Enforcement
- GIVEN a request for `maxDepth=15`
- WHEN `scouter_impact` is executed
- THEN the system MUST cap the depth at 10.

### Requirement: MCP Interface and Output
The system SHALL expose the impact analysis via an MCP tool and return a standardized JSON structure.
(Previously: New capability)

#### Scenario: Successful Impact Retrieval
- WHEN `scouter_impact(symbolName="Handle", filePath="internal/api.go", maxDepth=2)` is invoked
- THEN the response MUST be a JSON array of objects
- AND each object MUST contain `symbol`, `file`, and `distance`
- AND the array MUST be sorted by `distance` ascending.

#### Scenario: Symbol Not Found
- WHEN `scouter_impact` is called for a symbol that does not exist in the index
- THEN the system MUST return a `SymbolNotFoundError`.

### Requirement: Recursive CTE Contract
The data store implementation MUST use a Recursive Common Table Expression (CTE) to perform the upward traversal efficiently.
(Previously: New capability)

#### Scenario: SQL Execution Structure
- GIVEN a request for impact analysis
- WHEN the store executes the query
- THEN it MUST follow this structure:
  - **Anchor**: Select `caller_name`, `path` (as `caller_path`) where `callee_name = ?` and `callee_path = ?`.
  - **Recursive Step**: Join `calls` (c) with `impact` (i) on `c.callee_name = i.caller_name` AND `c.callee_path = i.caller_path`.
  - **Termination**: The depth MUST be checked in the recursive step (e.g., `i.depth < maxDepth`) to stop traversal.
  - **Final Selection**: The final SELECT MUST use `DISTINCT` to handle cycles and multiple call sites between the same symbols.
