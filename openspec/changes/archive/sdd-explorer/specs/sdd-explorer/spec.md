# SDD Explorer Specification

## Purpose
Provides structured access to project specifications, change records, and task status via MCP tools and resources, facilitating project trajectory discovery and management.

## Requirements

### Requirement: Explore SDD Tool
The system MUST provide a tool named `explore_sdd` that allows searching and filtering SDD artifacts (specs, proposals, tasks) within the `openspec/` hierarchy.

#### Scenario: List all active changes
- GIVEN the MCP server is running
- WHEN I call `explore_sdd` with no parameters
- THEN the tool MUST return a list of all active change directories in `openspec/changes/`
- AND the list MUST include the status of each change.

#### Scenario: Search specs by keyword
- GIVEN multiple specifications exist in `openspec/specs/`
- WHEN I call `explore_sdd` with `query="orchestration"`
- THEN the tool MUST return a list of specifications containing the keyword
- AND results MUST support pagination via `limit` and `offset`.

### Requirement: SDD Roadmap Resource
The system SHALL provide an MCP resource `scouter://sdd/roadmap` that summarizes the current project phase and trajectory.

#### Scenario: Access roadmap
- GIVEN the `openspec/state.yaml` exists and is valid
- WHEN I access the resource `scouter://sdd/roadmap`
- THEN the server MUST return a summary of the project state
- AND it MUST identify the current active phase (e.g., Discovery, Implementation, Verification).

### Requirement: SDD Tasks Resource
The system SHALL provide an MCP resource `scouter://sdd/tasks` that provides a structured view of pending and completed tasks.

#### Scenario: Access task list
- GIVEN the `openspec/tasks.md` or `sdd/tasks.md` (migrated) exists
- WHEN I access the resource `scouter://sdd/tasks`
- THEN the server MUST return a list of tasks grouped by their completion status.
