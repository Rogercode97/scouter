# Specification: Fix Staging Ledger Rollback & Data Loss

## Overview
Defines transactional contracts for the Staging Ledger and Evolution Engine to eliminate data loss, prevent `.scouter.tmp` file leaks, preserve original file modes, and unify commit orchestration into a single pipeline.

## Structural Target Symbols
| Symbol | Component | File | Role |
|---|---|---|---|
| `Ledger.Stage` | Staging Ledger | `internal/engine/ledger.go` | Stage mutation, capture baseline content and mode |
| `RippleEngine.Propagate` | Ripple Engine | `internal/engine/ripple.go` | Multi-file transformation caller |
| `EvolutionEngine.CommitLedger` | Evolution Engine | `internal/engine/evolution_engine.go` | Transactional commit orchestrator entry point |
| `Ledger.CommitStaged` | Staging Ledger | `internal/engine/ledger.go` | Staged commit entry point |
| `preparePatchStep` | Apply Engine | `internal/engine/evolution_engine.go` | Writes `.scouter.tmp` staging file |
| `applyPatchStep` | Apply Engine | `internal/engine/evolution_engine.go` | Promotes `.scouter.tmp` to target path |
| `apply.RollbackStep` | Apply Pipeline | `internal/engine/apply/stages.go` | Rollback contract interface |

## Requirements

### Requirement: Baseline Capture on Stage
`Ledger.Stage` MUST automatically populate `Patch.Original` and file mode from the filesystem when `Original` is unpopulated and the target file exists.

#### Scenario: Existing file staged with empty original
- GIVEN an existing file on disk at path `P` with content `C` and permissions `M`
- WHEN `Ledger.Stage` is called for path `P` with an empty `Original`
- THEN `Patch.Original` MUST be populated with content `C`
- AND original file mode `M` MUST be captured for restoration.

#### Scenario: Non-existent file staged
- GIVEN target path `P` does not exist on disk
- WHEN `Ledger.Stage` is called for path `P`
- THEN `Patch.Original` MUST remain empty
- AND the patch MUST be marked as a new file creation.

#### Scenario: Explicit original content preserved
- GIVEN a patch provided with non-empty `Original` string `O`
- WHEN `Ledger.Stage` is called for path `P`
- THEN `Patch.Original` MUST retain value `O` without being overwritten by disk reads.

### Requirement: Unified Transactional Commit & Mode Preservation
All staging ledger commits MUST execute through the unified `apply.Orchestrator` transaction pipeline and preserve existing file modes. Dueling commit paths MUST be merged.

#### Scenario: Commit preserves non-default file modes
- GIVEN a staged modification for file `P` with original mode `0755`
- WHEN `CommitLedger` or `CommitStaged` executes successfully
- THEN file `P` MUST contain the new content
- AND file `P` permissions MUST remain `0755`.

#### Scenario: Commit failure triggers atomic rollback
- GIVEN multiple staged patches and an I/O failure on applying the second patch
- WHEN `CommitStaged` or `CommitLedger` executes
- THEN all previously modified files MUST be restored to their original content and mode
- AND the staging ledger state MUST NOT be cleared.

### Requirement: Leak-Free Rollback & Non-Destructive File Restoration
`preparePatchStep` MUST implement `apply.RollbackStep`. Temporary `.scouter.tmp` files MUST be removed on abort or rollback. `applyPatchStep.Rollback()` MUST NEVER delete pre-existing source files.

#### Scenario: Prepare abort cleans temporary files
- GIVEN commit preparation creates `P.scouter.tmp`
- WHEN the preparation phase fails or context is cancelled
- THEN `preparePatchStep.Rollback()` MUST remove `P.scouter.tmp` from disk
- AND no temporary staging files MUST remain.

#### Scenario: Rollback restores existing file without deletion
- GIVEN an existing file `P` modified during `apply` phase
- WHEN `applyPatchStep.Rollback()` runs due to downstream failure
- THEN file `P` MUST NOT be removed via `os.Remove`
- AND file `P` MUST be restored to its exact `Original` content and mode.

#### Scenario: Rollback removes newly created file
- GIVEN a patch for a non-existent file `P` applied to disk
- WHEN `applyPatchStep.Rollback()` runs
- THEN file `P` MUST be removed from disk.
