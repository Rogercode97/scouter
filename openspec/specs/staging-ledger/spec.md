# Spec: Staging Ledger

## Description
The Staging Ledger capability provides a mechanism for atomic staging, diff generation, and persistence of code mutations. It allows for "dry-run" operations where changes are proposed and reviewed before being applied to the filesystem.

## Requirements

### Requirement: Hardened Integer Operations
The staging ledger MUST safely handle integer arithmetic for file offsets and lengths to prevent overflows during patch application or diff generation.

#### Scenario: Safe Offset Calculation
- GIVEN large file offsets in `ledger.go`
- WHEN performing addition or multiplication for buffer sizing
- THEN the system MUST use safe casting or overflow checks
- AND it MUST return an error if overflow is detected.

### Requirement: Explicit Error Handling
The ledger MUST NOT ignore return values from security-sensitive or I/O operations (e.g., file writes, permission changes).

#### Scenario: Unhandled Error Remediation
- GIVEN an I/O operation in the ledger
- WHEN the operation fails
- THEN the error MUST be captured and propagated to the caller
- AND Gosec MUST NOT flag unhandled errors in remediated sections.

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
