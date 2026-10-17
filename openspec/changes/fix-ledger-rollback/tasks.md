# Tasks: Fix Staging Ledger Rollback & Data Loss

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 180-240 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Baseline capture, atomic rollback & tmp cleanup | PR 1 | `go test -v ./internal/engine/... ./tests/...` | N/A (unit and integration tests) | Revert changes across `internal/engine` and `tests` |

## Phase 1: Baseline Capture & Patch Model

- [x] 1.1 Add failing tests in `internal/engine/ledger_test.go` verifying auto-population of `Original` and mode for existing files, `IsNew` flag for new files, and preservation of explicit `Original`.
- [x] 1.2 Extend `Patch` struct in `internal/engine/ledger.go` with `Mode os.FileMode` and `IsNew bool` fields.
- [x] 1.3 Implement baseline capture in `Ledger.Stage` in `internal/engine/ledger.go` to inspect target file on disk, read content and mode if unpopulated, and set `IsNew` when absent.

## Phase 2: Leak-Free Pipeline Steps & Orchestrator Rollback

- [x] 2.1 Add failing tests in `internal/engine/evolution_engine_test.go` and `internal/engine/apply/orchestrator_test.go` for `.scouter.tmp` deletion on abort and non-destructive rollback of existing files.
- [x] 2.2 Implement `Rollback()` on `preparePatchStep` in `internal/engine/evolution_engine.go` to remove `.scouter.tmp` staging files.
- [x] 2.3 Update `applyPatchStep` in `internal/engine/evolution_engine.go` to preserve file mode in `Run()` and in `Rollback()` unlink only if `IsNew`, else rewrite original content with mode.
- [x] 2.4 Update `Orchestrator.Execute` in `internal/engine/apply/orchestrator.go` and `RollbackPolicy` in `internal/engine/apply/rollback.go` to trigger rollback on prepare failure and clean staging files on apply failure.

## Phase 3: Unified Commit Pipeline

- [x] 3.1 Add failing tests in `internal/engine/ledger_staging_test.go` verifying atomic rollback preserves ledger state on commit error and preserves non-default file modes (`0755`).
- [x] 3.2 Refactor `Ledger.CommitStaged` in `internal/engine/ledger.go` to execute through `apply.Orchestrator`, clearing ledger only on commit success.
- [x] 3.3 Unify `EvolutionEngine.CommitLedger` in `internal/engine/evolution_engine.go` to route through the unified commit orchestration pipeline.

## Phase 4: Integration Verification

- [x] 4.1 Add end-to-end integration tests in `tests/ripple_integration_test.go` verifying `RippleEngine.Propagate` through commit and rollback preserves existing files and cleans temporary files.
- [x] 4.2 Verify all engine and integration tests pass with `go test ./...` against `openspec/changes/fix-ledger-rollback/spec.md` (read-only).

## Phase 5: Reliability Hardening (4R Remediation)

- [x] 5.1 Permission Denied on Read-Only Rollback (`internal/engine/evolution_engine.go`): Ensure destination file is writable (`os.Chmod(a.path, 0600)`) before rewriting original content in `applyPatchStep.Rollback()`.
- [x] 5.2 Explicit Error on Baseline Failure (`internal/engine/ledger.go`): Return unexpected stat/read errors immediately in `Ledger.Stage()`.
- [x] 5.3 Deterministic Staged Ordering (`internal/engine/ledger.go`): Sort staged file paths alphabetically (`sort.Strings`) in `Ledger.CommitStaged()`.
- [x] 5.4 Preserve Root Error in Orchestrator (`internal/engine/apply/orchestrator.go`): Join root and rollback errors with `errors.Join` in `Orchestrator.Execute()`.
- [x] 5.5 Best-Effort Rollback (`internal/engine/apply/rollback.go`): Accumulate rollback errors with `errors.Join` and continue rollback of all remaining steps in `ExecuteRollback()`.
- [x] 5.6 Comprehensive Unit Tests: Add test cases across `evolution_engine_test.go`, `ledger_test.go`, and `orchestrator_test.go` covering all 5 findings.

