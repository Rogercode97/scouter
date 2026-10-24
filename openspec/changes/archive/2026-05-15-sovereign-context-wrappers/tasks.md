# SDD Tasks: Sovereign Context Wrappers

## Phase 1: Foundation (SovereignContext and Constants)
- [x] **Task 1.1**: Create `internal/display/sovereign.go`.
- [x] **Task 1.2**: Define `SovereignState` type (`HOT`, `WARM`, `COLD`) and protocol constants (`#!SOV/1`).
- [x] **Task 1.3**: Define `SovereignContext` interface and `SovereignWrapper` struct.

## Phase 2: State Machine (Hot/Warm/Cold logic)
- [x] **Task 2.1**: Implement state transition logic in `SovereignWrapper`.
- [x] **Task 2.2**: Modify `internal/display/density.go` to allow `MUNCHEncoder` to be state-aware (or wrap it effectively).
- [x] **Task 2.3**: Implement formatting for `HOT` (full), `WARM` (MUNCH), and `COLD` (metadata) states.

## Phase 3: ULMEN (Hashing and Validation)
- [x] **Task 3.1**: Implement SHA-256 hashing logic for `COLD` state frames.
- [x] **Task 3.2**: Implement validation function to check hashes against the current symbol graph.

## Phase 4: ACCP (Frame and Window Management)
- [x] **Task 4.1**: Implement `ACCP` struct to manage a sliding window of context frames.
- [x] **Task 4.2**: Implement token threshold logic to trigger state transitions (`HOT` -> `WARM` -> `COLD`).

## Phase 5: Integration (MCP plumbing)
- [x] **Task 5.1**: Integrate `SovereignWrapper` into the MCP handlers (e.g., `internal/mcp/handle_analysis.go`).
- [x] **Task 5.2**: Write unit tests for `sovereign.go` covering state transitions and ULMEN hashing.
- [x] **Task 5.3**: Run full test suite to ensure 100% pass rate.