# Specification: Public Release Security Hardening

## Overview
This specification defines the strict behavioral requirements and scenarios for the "Public Release Security Hardening" change, enforcing Path Jail, Anti-Symlink guards, FTS5 SQL injection immunity, and DoS/Recursion protections.

## NEW Requirements

### Requirement: Security Boundary and Resource Protection

**Description**: The system MUST enforce strict boundaries on file operations, prevent SQL injection in search tools, and impose strict limits on resource-intensive queries and recursive traversals to ensure stability and security.

#### Scenario: Path Jail Enforcement
**Given** an active Scouter session operating within a designated repository root
**When** a request attempts to read a file with an absolute or relative path outside the repository root (e.g., `/etc/passwd` or `../../secret.txt`)
**Then** the operation MUST be blocked
**And** the system MUST return a security error indicating an out-of-bounds path access

#### Scenario: Symlink Attack Prevention
**Given** an active Scouter session
**When** a request attempts to access a file via a symlink pointing to a location outside the repository jail
**Then** the operation MUST be blocked
**And** the system MUST return a security error indicating an invalid or out-of-bounds symlink target

#### Scenario: FTS5 SQL Injection Neutralization
**Given** the `scouter_search` MCP tool is available
**When** a search query contains malicious or malformed FTS5 input (e.g., `" * OR 1=1`)
**Then** the input MUST be properly sanitized or neutralized
**And** the query MUST NOT cause SQL syntax errors
**And** the query MUST NOT result in data leakage beyond the intended search scope

#### Scenario: DoS Guardrails for Large Result Sets
**Given** a query execution that matches thousands of rows
**When** the query is executed against the database
**Then** the returned result set MUST be capped at a maximum of 500 rows
**And** the system MUST prevent buffer or memory exhaustion

#### Scenario: Recursion Depth Limits for Impact Analysis
**Given** the `scouter_impact` tool is executing an analysis
**When** the requested recursion depth exceeds 10
**Then** the system MUST implicitly cap the effective recursion depth at exactly 10
**And** the operation MUST complete using the capped depth value without processing further levels
