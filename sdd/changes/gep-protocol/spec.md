# Specification: Genome Evolution Protocol (v6.0)

## NEW Requirements

### Requirement: Scouter Evolve MCP Tool Execution
The `scouter_evolve` tool MUST orchestrate the self-modification of the Scouter codebase based on an input proposal, applying changes and verifying them autonomously.

**Scenario: Successful Evolution**
GIVEN the `scouter_evolve` tool is called
AND an `evolution_proposal` string is provided
WHEN the tool processes the proposal using MCP Sampling
THEN it MUST generate the exact code changes required to implement the proposal
AND it MUST apply the changes to the Scouter source code
AND it MUST execute `just build` and `go test ./...`
AND the verification MUST succeed
AND it MUST return a message indicating Scouter has evolved and requires a restart.

**Scenario: Failed Verification and Rollback**
GIVEN the `scouter_evolve` tool is called
AND the generated code changes are applied
WHEN the verification step executes `just build` and `go test ./...`
AND the verification fails
THEN it MUST rollback the changes using the atomic `.bak` mechanism
AND it MUST return a failure message indicating the rollback occurred.
