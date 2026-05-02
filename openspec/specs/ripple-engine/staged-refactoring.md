# Staged Refactoring Specification

## Purpose
Provide developers with fine-grained control over multi-file transformations, allowing for review and selective application.

## Requirements
### Requirement: Selective Commit
The system MUST allow the user to select which file patches from the ripple ledger to commit to the physical filesystem.
#### Scenario: Partial Refactor Commit
- GIVEN a ledger with 3 staged file transformations
- WHEN the user selects only 2 files for commitment
- THEN the system MUST apply only those 2 patches
- AND the ledger MUST retain the 3rd patch in a STAGED state or discard it based on user preference.

### Requirement: Transactional Rollback
The system MUST support an atomic rollback of all staged changes in a ripple transaction if any part of the process fails or is cancelled.
#### Scenario: Undo Full Ripple
- GIVEN a completed ripple transformation in the ledger
- WHEN the user invokes the rollback command before final commit
- THEN the system MUST restore all affected files to their original state using the ledger snapshots.
