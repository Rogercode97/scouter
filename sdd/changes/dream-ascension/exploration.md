## Exploration: Dream Ascension (Wave 11.1)

### Current State
Scouter's "Dream" engine is a manual MCP tool (`scouter_dream`) that extracts recent observations from Engram and distills them into ADRs, bug fixes, and patterns. It operates in isolation from the main conversation loop and requires explicit invocation.

### Affected Areas
- `internal/mcp/handle_dream.go` — Current entry point for distillation.
- `internal/mcp/handlers.go` — Needs a hook to trigger passive distillation.
- `internal/domain/memory/service.go` — Needs logic for "Passive Memory Capture".
- `internal/adapters/llm/mcp_distiller.go` — Needs to support "forked" context injection.

### Approaches

1. **Passive Engram Extraction (The "Hakaishin" Way)** — Implement a turn-end hook that forks the current context and extracts memories automatically, injecting them directly into Engram observations.
   - Pros: Zero user friction; persistent technical memory; utilizes prompt cache; does not pollute project source.
   - Cons: Higher Ki (token) consumption if not throttled.
   - Effort: Medium

2. **Structural Memory Anchoring** — Use Scouter's AST intelligence to link memories directly to code symbols (functions/classes) during distillation and store them in Engram.
   - Pros: High-fidelity context for the Linker; eliminates "hallucinated" memory scopes in the DB.
   - Cons: Requires tight integration between the Distiller and the Store/Symbol index.
   - Effort: High

### Recommendation
**Approach 1 (Passive Engram Capture)**: Implement a passive hook that runs at the end of significant tasks to distill memories and inject them directly into Engram. This preserves the project's source code (no GEMINI.md changes) while ensuring the agent's persistent memory is always synchronized with recent discoveries.

### Risks
- **Context Bloat**: Background extractions might consume too much of the Ki budget.
- **Race Conditions**: Updating SQLite memory while the main agent is reading/writing.
- **Database Growth**: Automated saves could bloat the Engram database if not deduplicated.

### Ready for Proposal
Yes. The next step is to create a formal Proposal and Spec for "Dream Ascension: Passive Memory Sovereignty".
