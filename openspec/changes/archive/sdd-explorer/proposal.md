# Proposal: Sovereign SDD Explorer

## Intent

Resolve fragmentation between `sdd/` and `openspec/` by unifying project tracking into a structured, MCP-native interface. This addresses the "branding crisis" and technical debt in agent-driven exploration, reducing Ki (token) consumption during discovery phases.

## Scope

### In Scope
- Unification of `sdd/` artifacts into the `openspec/` hierarchy.
- MCP Resource `scouter://sdd/roadmap`: Summarizes project trajectory.
- MCP Resource `scouter://sdd/tasks`: Structured view of pending/completed tasks.
- MCP Tool `explore_sdd`: Search and filter capabilities for specs and change records.
- Refactoring `internal/mcp/handlers.go` to support SDD-specific queries.

### Out of Scope
- Implementation of a full web UI for SDD.
- Automatic conversion of legacy text notes to structured YAML (manual migration).
- Integration with external task trackers (Jira/GitHub Issues).

## Capabilities

### New Capabilities
- `sdd-explorer`: Provides structured access to project specifications, change records, and task status via MCP.

### Modified Capabilities
- `static-resources`: Extend existing resource registration to include SDD-specific endpoints.

## Approach

1. **Unification**: Move valid `sdd/` content to `openspec/`.
2. **Logic Layer**: Implement `internal/engine/sdd.go` to parse `openspec/` artifacts using existing patterns.
3. **MCP Integration**: Register the `explore_sdd` tool and `scouter://sdd/*` resources in `internal/mcp/handlers.go` and `internal/mcp/resources.go`.
4. **Pagination**: Implement Wave 12.0 pagination (`limit`, `offset`) for the explorer tool.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/mcp/handlers.go` | Modified | Addition of `explore_sdd` tool handler. |
| `internal/mcp/resources.go` | Modified | Addition of `scouter://sdd/` resource endpoints. |
| `internal/engine/sdd.go` | New | Domain logic for SDD artifact parsing and querying. |
| `sdd/` | Removed | Deprecated in favor of `openspec/`. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Breaking MCP schema | Low | Use standard SDK types and validate with `make test`. |
| Parsing errors on legacy MD | Med | Implement robust, failure-tolerant markdown parsing. |
| Impact on `Server` struct | Low | Scouter Impact analysis shows 0 callers; isolation is high. |

## Rollback Plan

1. Revert changes to `internal/mcp/`.
2. Restore `sdd/` directory from git history.
3. Delete `internal/engine/sdd.go`.

## Dependencies

- Go 1.25.0
- MCP Go SDK

## Success Criteria

- [ ] `explore_sdd` tool returns structured results for existing specs.
- [ ] `scouter://sdd/roadmap` correctly identifies current project phase.
- [ ] Zero mentions of legacy `sdd/` directory in code.
- [ ] `make test` passes with 100% success on MCP handlers.
