# Predictive Testing Spec

## Feature: Predictive Testing
As a developer, I want to know which tests are affected by my changes so I can run only the necessary tests.

## Scenarios

### Scenario: Find tests for a modified function
Given a function `ProcessData` in `internal/engine/processor.go`
And a test `TestProcessData` in `internal/engine/processor_test.go` that calls `ProcessData`
When I run `scouter predict internal/engine/processor.go:ProcessData`
Then I should see `internal/engine/processor_test.go:TestProcessData` in the output

### Scenario: Indirect dependency
Given function `A` calls function `B`
And `TestB` calls function `B`
When I change `A`
Then `TestB` should NOT be flagged (unless it also calls A)
But if `TestA` calls `A`, it SHOULD be flagged.
