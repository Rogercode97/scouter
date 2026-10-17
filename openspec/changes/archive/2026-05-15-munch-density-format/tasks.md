# SDD Tasks: MUNCH Density Format Implementation

## Phase 1: Core Implementation (Sequential)

### Task 1.1: Define MUNCH Core Architecture
- **Action**: Create `internal/display/density.go`. Define `MUNCHEncoder` struct with `io.Writer` and path map. Define protocol constants.
- **Requirement**: Versioned Protocol Header, Deterministic Streaming
- **Status**: done [x]

### Task 1.2: Implement Path Interning
- **Action**: Add logic to `MUNCHEncoder` to track paths and emit `@ID:path` legend entries upon first encounter.
- **Requirement**: Path Interning (Legend Section)
- **Status**: done [x]

### Task 1.3: Symbol and Caller Encoding
- **Action**: Implement `EncodeSymbol(s Symbol)` and `EncodeCaller(c Caller)` using `S` and `C` tags.
- **Requirement**: Tagged Tabular Rows
- **Status**: done [x]

### Task 1.4: Intelligence Metric Encoding
- **Action**: Implement encoding methods for PageRank (`R`), Churn (`K`), and Critical Symbols (`X`).
- **Requirement**: Intelligence Data Tags
- **Status**: done [x]

## Phase 2: Testing & Benchmarking (Parallelizable)

### Task 2.1: Unit Test Suite
- **Action**: Create `internal/display/density_test.go`. Test header, interning, and basic tag generation.
- **Requirement**: Versioned Protocol Header, Path Interning
- **Status**: done [x]

### Task 2.2: Intelligence Encoding Tests
- **Action**: Test `R`, `K`, and `X` tag generation with mock data.
- **Requirement**: Intelligence Data Tags
- **Status**: done [x]

### Task 2.3: Token Efficiency Benchmark
- **Action**: Compare MUNCH output size vs JSON for a large result set (~100 symbols).
- **Requirement**: N/A (Internal Performance Goal)
- **Status**: done [x]

## Phase 3: MCP Integration (Sequential)

### Task 3.1: Analysis Handler Integration
- **Action**: Update `internal/mcp/handle_analysis.go` (`handleSearch`, `handleCallers`) to accept `format` argument and use `MUNCHEncoder`.
- **Requirement**: Format Selection Argument
- **Status**: done [x]

### Task 3.2: Intelligence Handler Integration
- **Action**: Update `internal/mcp/handle_intelligence.go` (`handleIntelligence`) to support MUNCH format.
- **Requirement**: Format Selection Argument
- **Status**: done [x]

## Phase 4: Verification & Finalization

### Task 4.1: End-to-End Integration Test
- **Action**: Verify MUNCH output via real MCP tool calls. Check header presence and interning correctness.
- **Requirement**: Deterministic Streaming
- **Status**: done [x]

### Task 4.2: Archive Change
- **Action**: Call `sdd-archive` to finalize the delta.
- **Requirement**: SDD Lifecycle
- **Status**: done [x]
