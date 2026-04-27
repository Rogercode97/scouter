# Spec: Hybrid Search (v3.5)

## Intent
Unify static code analysis (AST) with historical knowledge (Engram) to provide context-aware search results.

## Requirements

### Requirement: Hybrid Result Aggregation
Scouter MUST query both the local symbol store AND the Engram memory database for every hybrid search request.

#### Scenario: Successful Hybrid Search
GIVEN a symbol "GetImpact" exists in the project
AND an Engram memory exists with the text "GetImpact" and a "Learned" section
WHEN the user executes `scouter_hybrid_search` with query "GetImpact"
THEN the result MUST contain the AST symbol metadata
AND the result MUST contain a list of distilled Engram insights.

### Requirement: Insight Distillation
Scouter SHALL extract only the high-signal fields from Engram memories to avoid context bloat.

#### Scenario: Memory Cleansing
GIVEN an Engram search returns a memory with What, Why, and Learned fields
WHEN Scouter processes the hybrid result
THEN it MUST prioritize the "Learned" and "Why" fields
AND it SHOULD omit administrative metadata like local file paths if they match the AST result.

### Requirement: Project Scope Enforcement
Hybrid search MUST only return Engram memories associated with the current project.

#### Scenario: Cross-Project Isolation
GIVEN the current project is "scouter"
AND an Engram memory exists with the query term but belongs to "other-project"
WHEN a hybrid search is executed
THEN the memory from "other-project" MUST NOT be included in the results.
