## Exploration: SDD Explorer Tool

### Current State
Currently, SDD (Software Design Document) and OpenSpec artifacts are stored in the filesystem (`sdd/` and `openspec/`). Agents must manually navigate these directories using generic search and read tools. This leads to token waste and lack of structured visibility into the project's evolution and future roadmap.

### Affected Areas
- `internal/mcp/handlers.go` — New tool handler.
- `internal/mcp/resources.go` — New resources (optional).
- `internal/engine/sdd.go` — Logic for parsing and querying SDD/OpenSpec artifacts (to be created).

### Approaches
1. **MCP Tool: `explore_sdd`** — A single tool to query the SDD state.
   - Pros: High flexibility, easy for agents to use.
   - Cons: Requires a robust parser for the various MD formats.
   - Effort: Medium.

2. **MCP Resources: `scouter://sdd/`** — Expose artifacts as hierarchical resources.
   - Pros: Built-in discovery, follows MCP standards.
   - Cons: Less powerful than a tool for complex queries (e.g., "what are the pending tasks for Phase 3?").
   - Effort: Low.

3. **Hybrid Approach** — Expose resources for reading and a tool for querying.
   - Pros: Best of both worlds.
   - Cons: Slightly more implementation effort.
   - Effort: Medium-High.

### Recommendation
Implement a hybrid approach:
- Expose `scouter://sdd/roadmap` and `scouter://sdd/tasks` as resources.
- Implement an `explore_sdd` tool to search across specs and changes with pagination.

### Risks
- Syncing between `sdd/` and `openspec/` (need to decide on a single source of truth).
- Parsing complexity for non-standardized task lists.

### Ready for Proposal
Yes — I can prepare a proposal to implement the "Sovereign SDD Explorer" to unify tracking and improve agent autonomy.
