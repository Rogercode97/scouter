# Spec: Staging Ledger

## Description
The Staging Ledger capability provides a mechanism for atomic staging, diff generation, and persistence of code mutations. It allows for "dry-run" operations where changes are proposed and reviewed before being applied to the filesystem.

## Scenarios

### Scenario: Stage a file change
Given a source file exists
When I stage a change to the file with new content
Then the change MUST be stored in the staging area
And the change MUST NOT be applied to the disk yet.

### Scenario: Generate a unified diff
Given one or more changes are staged in the ledger
When I request a diff of the staged changes
Then the ledger MUST return a unified diff format string representing the changes
And the diff MUST accurately reflect the delta between disk and staging.

### Scenario: Persist staged changes
Given changes are staged in the ledger
When the ledger is requested to persist state
Then it MUST serialize the staged patches to `.scouter/staging/`
And it MUST be able to reload these patches after a restart.

### Scenario: Commit staged changes
Given changes are staged in the ledger
When I commit the staged changes
Then all staged patches MUST be written to their respective files on disk
And each file MUST be replaced atomically
And a failure during the batch MUST roll the whole batch back in process
And the staging area MUST be cleared upon success.

### Scenario: Commit is interrupted
Given a commit is in progress
And the process dies before the commit finishes
When the ledger is loaded again
Then the ledger MUST report the mission as an interrupted commit
And the ledger MUST name the files the commit already replaced and the files it did not
And the ledger MUST refuse to start another commit until the interrupted one is resolved
And the resolution MUST be an explicit decision, never an automatic one.

### Scenario: Resolve an interrupted commit by rolling back
Given the ledger reports an interrupted commit
When the operator rolls back
Then every file the commit already replaced MUST be restored to its original content
And a file that was created by the commit MUST be removed
And a file whose content changed after the interruption MUST NOT be overwritten
And the rollback MUST fail instead of overwriting it
And the journal MUST be removed once the tree is back to its previous state.

### Scenario: Resolve an interrupted commit by rolling forward
Given the ledger reports an interrupted commit
When the operator rolls forward
Then every file the commit did not replace yet MUST be written with its staged content
And the files the commit already replaced MUST be left untouched
And the staging area MUST be cleared
And the journal MUST be removed.

### Requirement: Durable Journal

The journal is the only record of a commit in progress, so it MUST survive the
failure it exists to describe.

#### Scenario: Journal write is atomic
- GIVEN a commit in progress
- WHEN the journal is written
- THEN the journal MUST be replaced atomically and never left torn

#### Scenario: Journal records the commit before the filesystem is touched
- GIVEN staged changes
- WHEN a commit starts
- THEN the journal MUST record the commit in progress and the files it is about to write
- AND the journal MUST keep that record until the commit reaches its end

### Requirement: Durable Replacement

#### Scenario: Applied file survives a power loss
- GIVEN a commit in progress
- WHEN a file is replaced
- THEN the content MUST be synced before the replacement
- AND the directory entry MUST be synced after it

### Scenario: Clear staging area
Given changes are staged in the ledger
When I clear the staging area
Then all staged patches MUST be discarded
And the disk MUST remain unchanged.

### Requirement: Thread-Safe Mutation Management

The Ledger and HealerEngine MUST be thread-safe. Access to shared state (staging area, in-memory caches) SHALL be synchronized using mutexes to prevent race conditions during concurrent test execution or parallel analysis.

#### Scenario: Concurrent Staging
- GIVEN multiple concurrent calls to `ledger.Stage()`
- WHEN the changes are processed
- THEN all changes MUST be stored correctly without data corruption or race detector warnings.

#### Scenario: Isolated Test State
- GIVEN concurrent tests running with `go test -race`
- WHEN each test uses its own Ledger/Store instance or properly synchronized shared state
- THEN the tests MUST pass consistently with zero race conditions.
