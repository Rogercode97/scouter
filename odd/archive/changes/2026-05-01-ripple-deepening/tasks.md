<tasks project="scouter" change="ripple-deepening">
  <task id="1.1" status="completed" type="task">
    <name>Define Core Interfaces and Types</name>
    <files>internal/engine/ripple.go</files>
    <action>
      Define PropagationStrategy, Validator, PropagationTask, and ValidationResult interfaces/structs as per design.
      Use SEARCH/REPLACE blocks.
    </action>
    <verify>go test -v ./internal/engine/ripple.go</verify>
    <commit>feat(engine): define ripple propagation strategy and validator interfaces</commit>
  </task>

  <task id="1.2" status="completed" type="task" depends_on="1.1">
    <name>Enhance Ledger for Staging</name>
    <files>internal/engine/ledger.go</files>
    <action>
      Add Stage, Unstage, and CommitStaged methods to Ledger.
      Ensure Ledger maintains in-memory patches before physical commit.
    </action>
    <verify>go test -v ./internal/engine/ledger.go</verify>
    <commit>feat(engine): enhance ledger with staging and selective commit capabilities</commit>
  </task>

  <task id="2.1" status="completed" type="task" depends_on="1.1">
    <name>Implement BFS Propagation Strategy</name>
    <files>internal/engine/ripple.go</files>
    <action>
      Extract existing BFS logic into a BFSPropagationStrategy implementation using Go 1.25 iter.Seq2.
      Traceability: Propagate Specification (Streamed Execution).
    </action>
    <verify>go test -v ./internal/engine/ripple.go</verify>
    <commit>feat(engine): implement BFS propagation strategy using iter.Seq2</commit>
  </task>

  <task id="2.2" status="completed" type="task" depends_on="1.1">
    <name>Implement Build and Test Validators</name>
    <files>internal/engine/ripple.go</files>
    <action>
      Implement BuildValidator (runs go build) and TestValidator (runs relevant tests).
      Traceability: Ripple Validation Specification (Build Integrity).
    </action>
    <verify>go test -v ./internal/engine/ripple.go</verify>
    <commit>feat(engine): implement build and test validators for ripple integrity</commit>
  </task>

  <task id="2.3" status="completed" type="task" depends_on="1.1">
    <name>Implement Centrality Validator</name>
    <files>internal/engine/ripple.go</files>
    <action>
      Implement CentralityValidator using AnalyzerEngine to detect spikes > 20%.
      Traceability: Ripple Validation Specification (Centrality Guard).
    </action>
    <verify>go test -v ./internal/engine/ripple.go</verify>
    <commit>feat(engine): implement centrality validator for ripple architectural guard</commit>
  </task>

  <task id="3.1" status="completed" type="task" depends_on="2.1, 1.2">
    <name>Refactor RippleEngine.Propagate</name>
    <files>internal/engine/ripple.go</files>
    <action>
      Update Propagate to coordinate strategy, transformer, and validator pipeline.
      Traceability: Staged Refactoring Specification (Transactional Rollback).
    </action>
    <verify>go test -v ./internal/engine/ripple.go</verify>
    <commit>refactor(engine): update ripple engine to use strategy and validation pipeline</commit>
  </task>

  <task id="3.2" status="completed" type="task" depends_on="3.1">
    <name>Update TruthEngine Integration</name>
    <files>internal/engine/truth.go</files>
    <action>
      Update TruthEngine.Propagate to handle the new staged Ledger and validation results.
    </action>
    <verify>go test -v ./internal/engine/truth.go</verify>
    <commit>feat(engine): update truth engine integration with deepened ripple engine</commit>
  </task>

  <task id="4.1" status="completed" type="task" depends_on="3.2">
    <name>Unit Test: Propagation Strategy</name>
    <files>internal/engine/ripple_test.go</files>
    <action>
      Create unit tests for BFSPropagationStrategy using a mock call graph.
    </action>
    <verify>go test -v ./internal/engine/ripple_test.go</verify>
    <commit>test(engine): add unit tests for BFS propagation strategy</commit>
  </task>

  <task id="4.2" status="completed" type="task" depends_on="3.2">
    <name>Integration Test: Full Staged Ripple</name>
    <files>tests/ripple_integration_test.go</files>
    <action>
      Implement end-to-end integration test for rename refactor with validation.
      Traceability: Staged Refactoring Specification (Selective Commit).
    </action>
    <verify>go test -v ./tests/ripple_integration_test.go</verify>
    <commit>test(integration): add end-to-end integration test for staged ripple</commit>
  </task>
</tasks>
