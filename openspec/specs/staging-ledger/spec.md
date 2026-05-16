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
And the staging area MUST be cleared upon success.

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
