# Propagate Specification (Deepened)

## Purpose
Traces the blast radius of a symbol and applies transformations across the project.

## Requirements
### Requirement: Streamed Execution
The system MUST use streaming iterators (Go 1.25 `iter.Seq`) to propagate changes, ensuring constant memory usage regardless of the blast radius size.
#### Scenario: Large Scale Ripple
- GIVEN a symbol with 500+ callers across 100 files
- WHEN the propagate command is executed
- THEN the system MUST process the transformations in a streamed fashion
- AND the peak memory usage SHOULD NOT exceed a defined threshold (e.g., 50MB for metadata).

### Requirement: Deterministic Caller Resolution
The system MUST resolve callers using the LSP-aware `ImpactEngine` to ensure all symbolic references are identified across all file boundaries.
#### Scenario: Cross-Package Rename
- GIVEN a public function `Sum` in package A used in package B
- WHEN `Sum` is renamed to `Add` via RippleEngine
- THEN the system MUST correctly identify and transform the calls in package B.
