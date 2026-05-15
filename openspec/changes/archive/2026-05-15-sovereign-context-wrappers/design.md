# Design: sovereign-context-wrappers

## Summary
- **Approach**: Introduced a state-aware `SovereignWrapper` that wraps the `MUNCHEncoder` to implement the Adaptive Context Compression Protocol (ACCP) and Unified Language-Model Evidence Network (ULMEN).
- **Key Decisions**: 
    - **SHA-256**: Chosen for ULMEN hashes due to standard support and collision resistance.
    - **Per-File Hashing**: Balances granularity with token efficiency by hashing `SovereignDelta` objects.
    - **Transient State**: ACCP state is managed in-memory during context generation, preserving the store as the immutable truth.
- **Files Affected**: 
    - `internal/display/sovereign.go` (New): Implementation of the wrapper and state machine.
    - `internal/display/density.go` (Modified): Adaptation of `MUNCHEncoder` for state-aware operations.
- **Testing Strategy**: Unit testing for state transitions and hashing integrity; integration testing for end-to-end context generation.

## Open Questions
- Should ULMEN hashes be persisted in the SQLite store to optimize repeat generation?
- Determination of the optimal token threshold (budget) for auto-transitioning between HOT, WARM, and COLD states.