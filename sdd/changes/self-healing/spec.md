# 🛡️ Specification: Atomic Self-Healing (v5.0)

## 1. Overview
This specification details the behavior of the new `scouter_self_heal` tool, which implements an atomic self-healing cycle. It relies on semantic diagnostic extraction from logs, model-based fix generation, and automated verification via tests to resolve code failures autonomously.

## 2. NEW Requirements

### Requirement: scouter_self_heal Input Reception
The `scouter_self_heal` tool MUST accept a raw error log string as its primary input.

**Scenario**: Processing a raw error log
  GIVEN the system is awaiting self-healing directives
  WHEN the tool is invoked with a raw error log string
  THEN it MUST capture and forward the log string to the diagnostic pipeline

### Requirement: Diagnostic Processing via Semantic Purification
The system MUST use Semantic Purification to identify the exact `File:Line` of the failure from the raw error log string.

**Scenario**: Extracting coordinates from failure logs
  GIVEN a raw error log containing standard failure stack traces
  WHEN Semantic Purification is applied to analyze the log
  THEN the system MUST successfully extract the `File:Line` coordinates representing the root cause of the error

### Requirement: Fix Generation via MCP Sampling
The tool MUST use MCP Sampling to request a fix from the model, supplying the specific failing code context identified during the diagnostic phase.

**Scenario**: Generating a fix for the identified failure
  GIVEN the `File:Line` coordinates have been successfully extracted
  WHEN the tool fetches the failing code context and initiates an MCP Sampling request
  THEN the model MUST return a proposed fix for the code block

### Requirement: Automated Verification
The tool MUST attempt to verify the proposed fix by running `go test` (or the equivalent test command) against the patched codebase.

**Scenario**: Verifying the proposed fix
  GIVEN a proposed fix has been returned by MCP Sampling
  WHEN the fix is temporarily applied to the codebase
  AND the system executes `go test` for verification
  THEN the system MUST capture the pass or fail outcome of the test suite

### Requirement: Atomic Output Status
The tool MUST return an atomic output: if verification passes, it MUST return the fixed code block with a `Verification: SUCCESS` status. If verification fails, it MUST return the original error with a `Verification: FAILED` status.

**Scenario**: Atomic output on verification pass
  GIVEN the verification step executed `go test` with the proposed fix
  WHEN the test execution exits with a success status code (0)
  THEN the tool MUST return the fixed code block
  AND append the status string `Verification: SUCCESS`

**Scenario**: Atomic output on verification failure
  GIVEN the verification step executed `go test` with the proposed fix
  WHEN the test execution exits with a failure status code (non-zero)
  THEN the tool MUST return the underlying error
  AND append the status string `Verification: FAILED`