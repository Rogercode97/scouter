# Sovereign Context Specification

## Purpose
Defines the protocol and lifecycle for wrapping agent context to achieve aggressive token reduction while maintaining semantic integrity.

## Requirements

### Requirement: Sovereign Wrapper Protocol Header
The system MUST wrap all context outputs with a versioned protocol header.
- GIVEN a context payload
- WHEN serialized via the Sovereign Wrapper
- THEN it MUST begin with the prefix `#!SOV/1`.
- AND it MUST include the current state metadata (`HOT`, `WARM`, or `COLD`).

### Requirement: Multi-State Context Transitions
The system MUST support three discrete context states.
- GIVEN a context frame
- WHEN in `HOT` state, it MUST contain the full content.
- WHEN in `WARM` state, it MUST contain MUNCH-encoded data and summaries.
- WHEN in `COLD` state, it MUST contain only file paths and ULMEN hashes.

### Requirement: Adaptive Context Compression Protocol (ACCP)
The system MUST manage context windows using a sliding frame management strategy.
- GIVEN an active context window
- WHEN the token count exceeds the threshold
- THEN the ACCP MUST transition older frames (e.g., `HOT` -> `WARM`).

### Requirement: ULMEN Semantic Validation
The system MUST use ULMEN hashes to prevent hallucinations.
- GIVEN a frame in `COLD` state
- WHEN accessed
- THEN the system MUST verify the hash against the current symbol graph.

### Requirement: Sovereign Display Integration
The system MUST provide a unified display interface in `internal/display/sovereign.go`.
- GIVEN a requirement to output context
- THEN the output MUST be processed by the `SovereignWrapper`.