# Proposal: Ripple Engine Deepening (Phase D)

## Intent
Evolve the `RippleEngine` from a shallow BFS-based propagator into a sovereign, high-fidelity refactoring engine. Current implementation lacks architectural validation and performance optimizations, risking technical debt accumulation during large-scale symbolic changes.

## Scope

### In Scope
- Extract BFS logic into domain-isolated `PropagationStrategy`.
- Implement a `Validator` pipeline for post-refactor integrity (Build, Test, Centrality).
- Refactor task orchestration to use Go 1.25 `iter.Seq` for streaming.
- Enhance `Ledger` to support staged previews and selective commits.

### Out of Scope
- Modifying the underlying MCP protocol.
- Implementing a visual GUI for refactoring previews.
- Changes to the persistent storage schema (SQLite).

## Capabilities

### New Capabilities
- `ripple-validation`: Enforces architectural invariants (e.g., centrality spikes < 20%) after symbolic propagation.
- `staged-refactoring`: Allows selective application and rollback of patches within a single ripple transaction.

### Modified Capabilities
- `propagate`: Transitioned from blocking I/O to streamed task execution with integrated validation.

## Approach
Implement a Hexagonal refactor where `RippleEngine` coordinates a `PropagationStrategy` for discovery, a `Transformer` for application, and a `Validator` pipeline for proof-of-correctness. Use `iter.Seq` to pipe tasks through these stages.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/engine/ripple.go` | Modified | Core orchestration logic and strategy extraction. |
| `internal/engine/ledger.go` | Modified | Enhanced for staged commits and selective rollback. |
| `internal/engine/truth.go` | Modified | Updated integration with the deepened engine. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Circular Ripple Deadlock | Low | Implement depth limits and cycle detection in Strategy. |
| Performance Regression | Med | Use streaming iterators and concurrent validation where safe. |

## Rollback Plan
`Ledger` maintains atomic snapshots of all affected files. If validation fails or the user rejects the staged changes, `ledger.Rollback()` restores the codebase to the pre-transaction state.

## Success Criteria
- [ ] Multi-file refactors pass build and tests automatically.
- [ ] Centrality spikes > 20% trigger a `SUCCESS_WITH_WARNING` status.
- [ ] Zero memory allocations for task queue management via `iter.Seq`.
