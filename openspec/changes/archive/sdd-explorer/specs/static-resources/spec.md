# Delta for Static Resources

## MODIFIED Requirements

### Requirement: Resource discovery

The system MUST allow clients to discover all available read-only resources, including architectural graphs, schemas, and SDD artifacts.
(Previously: Minimal discovery of dependency graph and MCP schema.)

#### Scenario: Resource discovery
- GIVEN the MCP server is running
- WHEN a client requests a list of resources
- THEN the server MUST include:
    - `file:///scouter/graph/dependencies`
    - `file:///scouter/schema/mcp`
    - `scouter://sdd/roadmap`
    - `scouter://sdd/tasks`
- AND the list SHOULD be expandable as new capabilities are added.

## ADDED Requirements

### Requirement: URI Scheme Support

The system MUST support both `file://` (for local file-based context) and `scouter://` (for virtual/computed domains) URI schemes.

#### Scenario: Register new URI scheme
- GIVEN the resource manager is initializing
- WHEN a new capability registers a `scouter://` URI
- THEN the system MUST accept the registration and route requests to the appropriate handler.
