# ADR 0003: Sovereign Orchestration (Scouter 🤝 Engram)

## Context
Scouter (Engineering Engine) and Engram (Persistent Memory) are both MCP servers. To achieve Wave 12.0 "Ascension", they must collaborate. Scouter provides technical action, while Engram provides historical truth.

## Decision
We will implement "Orchestration via Agent Sampling". Instead of tight coupling at the Go code level, Scouter will use the MCP Sampling protocol to request historical context from the Agent (Gemini), who will then query Engram.

### Integration Points:
1. **Context Compaction**: `CompactionEngine` results will be suggested for persistence in Engram.
2. **Shinigami Protocol**: `HealerEngine` will request "Historical Insights" from Engram before attempting fixes.
3. **Ripple Propagation**: `RippleEngine` will request "Architectural Constraints" (ADRs) from Engram before mass refactoring.

## Consequences
- **Pros**:
    - Zero coupling: Scouter doesn't need Engram's binary to function.
    - Safety: The Agent (Gemini) acts as a validator between both servers.
    - Scalability: Any other MCP memory server can replace Engram without changing Scouter's code.
- **Cons**:
    - Increased turn count for high-stakes operations (worth the Ki for precision).

## Status
Proposed (Wave 12.0 Final Phase)
