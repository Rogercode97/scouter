# Delta Spec: MCP Server Sovereignty Fix

## Description
This delta spec covers enhancements to the MCP Server to handle sampling fallbacks and integrate the new dry-run/staging and static resource capabilities.

## Scenarios

### Scenario: Graceful fallback on Sampling failure
Given the MCP server attempts a `sampling/createMessage` request
When the client returns a `-32601 Method not found` error
Then the server MUST NOT crash or return an error to the tool handler
And it MUST return a standard fallback message instructing manual review.

### Scenario: Dry-run execution for mutation tools
Given a mutation tool (e.g., `ripple_refactor`, `evolve`) is called
When the `dryRun` parameter is set to `true`
Then the tool MUST stage the changes in the ledger
And it MUST return the generated unified diff in its response
And it MUST NOT commit changes to disk.

### Scenario: Integrated Resource Links in tool results
Given a tool execution completes
When the result is returned to the client
Then the server SHOULD include links to relevant resources (e.g., staging diff) if applicable.
