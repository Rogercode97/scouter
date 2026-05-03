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
