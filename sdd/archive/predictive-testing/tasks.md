# Tasks: Predictive Testing (Wave 9.1)

## 🏗️ Phase 1: Foundation (Data & Logic)

- [ ] **Task 1.1: Implement Recursive CTE in Store**
  - **Files**: `internal/store/store.go`
  - **Description**: Add `GetAffectedTests` method to the `Store` interface and implementation. Use a Recursive CTE to find all callers (upward traversal) of a symbol ID that match the pattern `Test%`.
  - **Traceability**: [Scenario: Find tests for a modified function]

- [ ] **Task 1.2: Implement Predict Engine Logic**
  - **Files**: `internal/engine/predict.go`
  - **Description**: Create `Predict` function that takes a target string (file:symbol), resolves it to a symbol ID using the resolver, and queries the store for affected tests.
  - **Traceability**: [Scenario: Find tests for a modified function]

- [ ] **Task 1.3: Unit Test for Predict Engine**
  - **Files**: `internal/engine/predict_test.go`
  - **Description**: Add unit tests for the `Predict` logic using a mock store.
  - **Traceability**: [Scenario: Indirect dependency]

## 🔌 Phase 2: Wiring (MCP & CLI)

- [ ] **Task 2.1: Register MCP Predict Tool**
  - **Files**: `internal/mcp/handlers.go`
  - **Description**: Add `handlePredict` function and register the `scouter_predict` tool in the MCP server.
  - **Traceability**: [Scenario: Find tests for a modified function]

- [ ] **Task 2.2: Implement CLI Predict Command**
  - **Files**: `internal/cli/flags.go`, `internal/cli/cli.go`
  - **Description**: Add `predict` command to the CLI. Wire it to the engine's `Predict` function. Enforce `file:symbol` format.
  - **Traceability**: [Scenario: Find tests for a modified function]

## 🛡️ Phase 3: Verification

- [ ] **Task 3.1: Integration Test for Predictive Testing**
  - **Files**: `tests/predict_test.go`
  - **Description**: Create an end-to-end integration test that indexes a sample file, modifies it, and verifies `scouter predict` returns the correct test file.
  - **Traceability**: [Scenario: Find tests for a modified function]

- [ ] **Task 3.2: Final Verification**
  - **Description**: Run `go test ./...` and verify all tests pass, including the new predictive testing scenarios.
  - **Traceability**: [Scenario: Find tests for a modified function], [Scenario: Indirect dependency]
