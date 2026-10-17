# Spec: SDD Explorer

## Purpose

The SDD Explorer capability provides structured, searchable access to the OpenSpec-driven Development (SDD) artifacts. It allows agents and users to query the project's trajectory, current task status, and technical specifications through the MCP interface, reducing context noise and improving discovery efficiency.

## Requirements

### Requirement: SDD Exploration Tool

The system MUST provide a tool named `explore_sdd` that allows searching and filtering SDD artifacts (specs and changes) using pagination.

#### Scenario: Search specs by domain
- GIVEN a project with multiple specs in `openspec/specs/`
- WHEN I call `explore_sdd` with `query="ripple"`
- THEN the system MUST return a list of matching specs
- AND each result MUST include the domain name and a brief summary.

#### Scenario: Paginated results
- GIVEN a large number of specs
- WHEN I call `explore_sdd` with `limit=5` and `offset=0`
- THEN the system MUST return the first 5 matching artifacts
- AND indicate if more results are available.

### Requirement: SDD Roadmap Resource

The system MUST expose a resource at `scouter://sdd/roadmap` that summarizes the project's current phase and overall trajectory based on `openspec/` state.

#### Scenario: Access roadmap
- GIVEN the MCP server is running
- WHEN I access the resource `scouter://sdd/roadmap`
- THEN the system MUST return a structured summary of the project roadmap
- AND it MUST identify the current active SDD change if applicable.

### Requirement: SDD Tasks Resource

The system MUST expose a resource at `scouter://sdd/tasks` that provides a structured view of pending and completed tasks for the current project.

#### Scenario: Access tasks
- GIVEN the MCP server is running
- WHEN I access the resource `scouter://sdd/tasks`
- THEN the system MUST return a list of tasks categorized by status (TODO, DOING, DONE)
- AND it MUST include the task ID and description.
