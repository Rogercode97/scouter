# Spec: Scouter v7.0 (The Context OS)

## Intent
Adopt advanced Engram patterns to transform Scouter into a context-aware operating system for codebase intelligence, minimizing token usage and maximizing architectural memory.

## Requirements

### Requirement: Memory Evolution (Topic Key Upserting)
Scouter MUST use stable topic keys when saving analysis results to Engram to prevent redundant memories and enable technical evolution.

#### Scenario: Updating Symbol Risk
GIVEN a symbol "Run" has an existing risk record in Engram
WHEN `scouter_impact` is executed on "Run"
THEN Scouter MUST update the existing Engram entry using the `topic_key` "scouter/risk/Run"
AND it SHALL NOT create a duplicate memory entry.

### Requirement: Token Sovereignty (Progressive Disclosure)
Scouter tools SHALL return a high-density summary by default and only provide exhaustive details upon explicit request.

#### Scenario: Compact Impact Analysis
GIVEN a complex function with 50+ callers
WHEN `scouter_impact` is called without the `verbose` flag
THEN it MUST return only the `RiskScore`, `RiskLevel`, and a summary count of callers
AND it SHALL NOT return the full Mermaid graph or the detailed callers list.

#### Scenario: Exhaustive Impact Analysis
WHEN `scouter_impact` is called with `verbose: true`
THEN it MUST return the full technical payload including Mermaid graph and callers metadata.

### Requirement: Autonomous Learning (Passive Capture)
Scouter MUST automatically extract and anchor learnings from successful autonomous actions.

#### Scenario: Passive Healing Ingestion
GIVEN the `scouter_self_heal` tool successfully fixes a bug
WHEN the tool completes the verification loop
THEN Scouter MUST automatically invoke Engram's `mem_capture_passive` (or equivalent) to record the fix pattern
AND it SHALL include the "Learned" technical wisdom in the observation.
