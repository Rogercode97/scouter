# Feature: Semantic Purification (v4.0)

## Context
A new filter action `semantic_purify` is introduced to detect Go test failure patterns, fetch actual source code context using the `SourceResolver` interface, and inject a "🧬 SOURCE CONTEXT" block into the logs.

## NEW Requirements

### Requirement: REQ-SP-001: Pattern Detection for Test Failures
The system MUST detect standard Go test failure patterns (e.g., `file.go:123:`) within the output stream using regular expressions.

- **Scenario: Valid Go test failure output is processed**
  - **GIVEN** the `semantic_purify` filter action is active
  - **WHEN** the input stream contains a log line matching the regex pattern for a Go test failure (e.g., `pkg/file_test.go:45: error`)
  - **THEN** the system SHALL extract the file path and line number.

### Requirement: REQ-SP-002: Context Injection via SourceResolver
The system MUST use the `SourceResolver` to fetch code snippets for detected failures and inject them with a specific marker.

- **Scenario: Fetching and injecting source code snippet**
  - **GIVEN** a file path and line number extracted from a test failure
  - **WHEN** the system calls the `SourceResolver` for that location
  - **THEN** it MUST append the fetched snippet to the output, prefixed with a `🧬 SOURCE CONTEXT` block.

### Requirement: REQ-SP-003: Resilience against Resolver Failures
The system MUST remain stable and not crash if the `SourceResolver` fails or the file is unavailable.

- **Scenario: SourceResolver fails to fetch snippet**
  - **GIVEN** a file path and line number extracted from a test failure
  - **WHEN** the `SourceResolver` returns an error or fails to find the file
  - **THEN** the system MUST return the original log lines without modification
  - **AND** the system SHALL NOT crash or halt processing.

### Requirement: REQ-SP-004: SNR Focus (Signal-to-Noise Ratio)
The system MUST prioritize failed tests and SHALL NOT process or bloat successful log lines.

- **Scenario: Ignoring successful test logs**
  - **GIVEN** the `semantic_purify` filter action is active
  - **WHEN** the input stream contains standard or successful log lines without failure patterns
  - **THEN** the system SHALL return the original log lines unmodified without invoking the `SourceResolver`.
