# Spec: Static Resources

## Description
The Static Resources capability exposes architectural context and protocol metadata as read-only MCP resources. This reduces the need for expensive tool calls and provides instant context to the agent.

## Scenarios

### Scenario: Read Dependency Graph resource
Given the MCP server is running
When I access the resource `file:///scouter/graph/dependencies`
Then the server MUST return a summarized representation of the Global Call Graph
And the output MUST be truncated or summarized if it exceeds 25,000 characters.

### Scenario: Read MCP Schema resource
Given the MCP server is running
When I access the resource `file:///scouter/schema/mcp`
Then the server MUST return the JSON schema of all registered tools
And the schema MUST follow the MCP tool discovery format.

### Scenario: Resource discovery
Given the MCP server is running
When a client requests a list of resources
Then the server MUST include `file:///scouter/graph/dependencies` and `file:///scouter/schema/mcp` in the list.
