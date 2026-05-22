# CLAUDE.md

<!-- Generated for repository development workflows. Do not edit directly. -->

Before beginning work in this repository, read `AGENTS.md` and follow all scoped AGENTS guidance.

## Testing and Validation Commands

- Run engine unit tests: `rtk proxy go test ./internal/engine/...`
- Test impact engine metrics: `rtk proxy go test ./tests/ -run TestImpactEngine_Analyze_Mixed`
- Test Tree-sitter streaming parser: `rtk proxy go test ./internal/engine/ -run TestStreamWithTreeSitter`

## Key Patterns
- **Impact Metrics**: The blast radius integrates PageRank scores into the `Centrality` metric to accurately reflect topological importance.
- **Mermaid Generation**: Semantic distinction between `calls` (`-->`) and `implements` (`-.->`) edges must be maintained.
