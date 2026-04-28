# Proposal: predictive-testing

## Intent
Implement a new tool `scouter_predict` (MCP and CLI) that analyzes the current git diff, maps changed lines to AST symbols using the store, and recursively identifies affected tests using the Global Call Graph. This optimizes testing by focusing only on impacted areas, minimizing unnecessary execution and reducing Ki consumption.

## Scope

### In Scope
- Add queries in `internal/store` to find symbols by file and line range.
- Create prediction logic in `internal/engine` to trace symbols to tests using `link_type='call'` and `link_type='dynamic'`.
- Register the `predict` tool in `internal/mcp`.
- Add the `predict` subcommand in `cmd/scouter/main.go`.

### Out of Scope
- Automatic execution of the predicted tests (only identification is in scope).
- Deep integration with external CI/CD pipelines (focus is CLI/MCP only).

## Capabilities

### New Capabilities
- `predictive-testing`: Core capability to map changed files/lines to AST symbols and trace to test targets.

### Modified Capabilities
- `mcp-server`: Add new `predict` tool.
- `cli-commands`: Add new `predict` subcommand.

## Approach
1. **Line Mapping**: Read the current git diff. For each modified file and line range, query `internal/store` to identify modified AST symbols.
2. **Blast Radius Analysis**: Leverage the Global Call Graph (SQLite database) traversing `link_type='call'` and `link_type='dynamic'` recursively to find dependent symbols until reaching test functions.
3. **Engine Implementation**: Encapsulate the traversal logic within a new module in `internal/engine` (e.g., `predict.go`).
4. **Interface Wrapper**: Expose the core engine functionality via the `internal/mcp` framework (`scouter_predict`) and the main CLI binary (`scouter predict`).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/store` | Modified | Add query logic for symbol resolution by line range. |
| `internal/engine` | New | Add prediction engine module and Graph traversal. |
| `internal/mcp` | Modified | Register `scouter_predict` tool in `handlers.go`/`prompts.go`. |
| `cmd/scouter` | Modified | Add `predict` command in `main.go`. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Incomplete map tracing via dynamic calls | Medium | Ensure graph captures dynamic links; gracefully fallback to file-level testing if necessary. |
| Excessive query times on large codebases | Low | Use optimized recursive CTE queries in SQLite for the call graph traversal. |

## Rollback Plan
Revert the commit introducing the `predict` feature. Since the logic is additive, dropping the MCP registration and CLI subcommand completely removes execution paths, leaving existing functions uncompromised.

## Dependencies
- Pre-populated Global Call Graph via `scouter_index` (must run prior to prediction).

## Success Criteria
- [ ] Modifying an inner utility function correctly identifies its tests in `scouter predict` output.
- [ ] `scouter_predict` MCP tool properly returns a valid list of tests.
- [ ] `internal/engine` tests validate the correctness of the traversal logic.