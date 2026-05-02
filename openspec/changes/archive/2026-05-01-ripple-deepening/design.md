# Design: Ripple Engine Deepening (Phase D)

## Technical Approach
Evolve the BFS-based `RippleEngine` into a strategy-driven pipeline. The engine will coordinate discovery via a `PropagationStrategy`, application via the `Transformer`, and verification via a `Validator` pipeline. All task orchestration will use Go 1.25 `iter.Seq` for allocation-free streaming.

## Architecture Decisions
| Decision | Choice | Rationale |
|----------|--------|-----------|
| Propagation Logic | `PropagationStrategy` Interface | Decouples traversal (BFS/DFS) from orchestration. |
| Verification | `Validator` Pipeline | Ensures post-fix integrity (build/test/centrality). |
| Data Flow | Go 1.25 `iter.Seq` | Constant memory usage for large-scale refactors. |
| Atomic Ops | Staged `Ledger` | Allows selective commits and transactional rollback. |

## Data Flow
```
User -> TruthEngine -> RippleEngine
                         |
                         v
                PropagationStrategy (Discover) -> iter.Seq2[Task, error]
                         |
                         v
                Transformer (Apply) -> Ledger (Staged Patches)
                         |
                         v
                Validator Pipeline (Verify) -> Build, Test, Centrality
                         |
                         v
                User (Review) -> Ledger (CommitStaged) -> Filesystem
```

## File Changes
| File | Action | Description |
|------|--------|-------------|
| `internal/engine/ripple.go` | Modify | Extract `PropagationStrategy` and integrate `Validator`. |
| `internal/engine/ledger.go` | Modify | Add `Stage`, `Unstage`, and `CommitStaged` methods. |
| `internal/engine/truth.go` | Modify | Update `Propagate` call to handle staged results. |

## Interfaces / Contracts

```go
// PropagationStrategy defines how to discover affected symbols.
type PropagationStrategy interface {
    Discover(ctx context.Context, startSymbol string, depth int) iter.Seq2[PropagationTask, error]
}

// Validator defines a check to be run after staging changes.
type Validator interface {
    Validate(ctx context.Context, ledger *Ledger) (ValidationResult, error)
}

// PropagationTask represents a single unit of refactoring.
type PropagationTask struct {
    File           string
    Symbol         string
    Transformation string
}
```

## Testing Strategy
| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `PropagationStrategy` | Mock call graph to verify discovery depth. |
| Unit | `Validator` | Mock build failures and centrality spikes. |
| Integration | `RippleEngine` | End-to-end rename with full validation pipeline. |

## Migration / Rollout
No data migration required. Existing `RippleEngine` calls in `internal/mcp` will be updated to the new staged workflow.
