# ADR 0003: Persistence Orchestration (Scouter 🤝 Engram)

## Status
Accepted

## Context
Scouter (Analysis Engine) and Engram (Persistent Memory) are both MCP servers. To achieve better collaboration, they must integrate their capabilities. Scouter provides technical analysis, while Engram provides historical data.

## Decision
We will implement orchestration via agent-mediated communication. Instead of tight coupling at the Go code level, Scouter will use the MCP Sampling protocol to request historical context from the agent, who will then query Engram as needed.

### Integration Points:
1. **Context Compaction**: Results from the compaction engine will be suggested for persistence in the memory layer.
2. **Diagnostic Protocol**: The healer engine will request historical insights from the memory layer before attempting fixes.
3. **Change Propagation**: The refactoring engine will request relevant architectural constraints or prior decisions from the memory layer before mass refactoring.

## Consequences
- **Pros**:
    - Decoupled architecture: Scouter does not depend on the specific implementation of the memory layer.
    - Safety: The agent acts as a validator and filter between both servers.
    - Scalability: The memory layer can be replaced or upgraded independently of Scouter.
- **Cons**:
    - Increased interaction count for complex operations.
