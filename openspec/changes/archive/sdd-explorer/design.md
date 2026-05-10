# Design: SDD Explorer

## Technical Approach
The SDD Explorer unifies project tracking by moving legacy `sdd/` artifacts into the structured `openspec/` hierarchy and exposing them via a new `SDDEngine`. This engine will be integrated into the `TruthEngine` and exposed through MCP tools and resources, following the project's Hexagonal Architecture and "Context-First I/O" principles.

## Architecture Decisions

### Decision: Specialized SDD Engine
**Choice**: Create a dedicated `SDDEngine` in `internal/engine/sdd.go`.
**Alternatives considered**: Adding logic directly to `TruthEngine` or `SearchEngine`.
**Rationale**: Maintains the established pattern of specialized engines and keeps `TruthEngine` as a clean orchestrator.

### Decision: Resource URI Scheme
**Choice**: Use `scouter://sdd/roadmap` and `scouter://sdd/tasks`.
**Alternatives considered**: `file:///openspec/...`
**Rationale**: Provides a higher-level, project-oriented abstraction instead of raw file access, consistent with `scouter://workspace`.

### Decision: Pagination for Explorer Tool
**Choice**: Implement `limit` and `offset` parameters for `explore_sdd`.
**Alternatives considered**: Returning full lists.
**Rationale**: Adheres to Wave 12.0 standards for performance and context window efficiency (Ki preservation).

## Data Flow
1. **MCP Handler**: Receives `explore_sdd` call or resource request.
2. **TruthEngine**: Routes request to `SDDEngine`.
3. **SDDEngine**: 
    - Scans `openspec/changes/` and `openspec/specs/`.
    - Parses `openspec/state.yaml` for roadmap info.
    - Aggregates tasks from `openspec/tasks.md`.
4. **Response**: Returns structured JSON or Markdown content.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/engine/sdd.go` | Create | Domain logic for SDD artifact parsing and querying. |
| `internal/engine/truth.go` | Modify | Add `SDDEngine` field and methods to `TruthEngine`. |
| `internal/mcp/handlers.go` | Modify | Add `ExploreSDDParams` and `handleExploreSDD`. |
| `internal/mcp/resources.go` | Modify | Register `scouter://sdd/roadmap` and `scouter://sdd/tasks`. |
| `internal/mcp/server.go` | Modify | Initialize `SDDEngine` and register `explore_sdd` tool. |
| `openspec/tasks.md` | Create | Unified task list (migrated from `sdd/tasks.md`). |

## Interfaces / Contracts

```go
type ExploreSDDParams struct {
	Query  string `json:"query,omitempty"`
	Type   string `json:"type,omitempty"` // spec, change, task
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type SDDRoadmap struct {
	Phase      string   `json:"phase"`
	Trajectory []string `json:"trajectory"`
	Changes    []string `json:"active_changes"`
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | SDD Parsing | Test `SDDEngine` with various `openspec/` file structures. |
| Integration | MCP Handlers | Use `server_test.go` to verify tool and resource output. |
| Verification | Scouter Strike | Run `scouter strike` to ensure no regressions in core AST tools. |

## Migration / Rollout
- Manual migration of `sdd/tasks.md` to `openspec/tasks.md`.
- Update `openspec/state.yaml` to include initial trajectory data.
- The `sdd/` directory will be deleted after verification.

## Open Questions
- [ ] Should `explore_sdd` support regex search or just keyword matching?
- [ ] Do we need to support legacy `sdd/` paths for backward compatibility during transition?
