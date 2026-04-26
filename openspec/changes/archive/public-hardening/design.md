# 🛡️ Technical Design: Public Release Security Hardening

## 🎯 Technical Approach
Implement robust security barriers to safely expose Scouter to public, untrusted users. This involves enforcing Path Jail mechanisms, robust FTS5 SQL sanitization, execution limit guardrails, and finalizing the MIT licensing for maximum adoption.

## 🏛️ Architecture Decisions

### 1. Path Jail Implementation
*   **Choice**: Use `filepath.EvalSymlinks` and `filepath.Abs` combined with a strictly enforced Base Directory anchor.
*   **Rationale**: Prevents Directory Traversal and symlink attacks. By deriving the anchor from `GetRepoRoot` (e.g., detecting `.git` or `go.mod`), we lock all file operations (reads/writes) strictly within the current repository boundary. Any resolved path must have the Base Directory as a prefix.
*   **Alternatives**: Relative path checks (prone to bypasses).

### 2. SQL FTS5 Sanitization (`SanitizeFTS`)
*   **Choice**: Introduce a strict `SanitizeFTS(query string) string` helper function.
*   **Rationale**: SQLite FTS5 queries have special control characters (`*`, `^`, `"`, `OR`, `AND`). Untrusted user input can cause syntax errors or unintended search manipulation. The `SanitizeFTS` function will aggressively escape internal double quotes and wrap all search terms in double quotes, neutralizing control characters.
*   **Alternatives**: Parameterized queries (do not work for FTS syntax structures).

### 3. DoS Execution Guardrails
*   **Choice**: Implement a strict 10-depth limit for Recursive CTEs and a hard cap of 500 rows for MCP tool outputs.
*   **Rationale**: Protects against complex queries exhausting memory or CPU. MCP `Server.sendResponse` will truncate outputs exceeding the limit.
*   **Alternatives**: Timeouts (less predictable than row limits).

### 4. License Finalization
*   **Choice**: Adopt the MIT License.
*   **Rationale**: Maximizes compatibility with the open-source ecosystem, particularly for a tool aiming for broad integration in agentic workflows.

## 🔀 Data Flow

```ascii
[Untrusted Input] ---> (MCP Handler)
                              |
                              v
                        [SanitizeFTS] ---> (Safe FTS Query) ---> [Store]
                              |
                              v
                        [ValidatePath] ---> (EvalSymlinks + RepoRoot Check)
                              |
                              v
                      [Row/Depth Guard] ---> (Truncated/Safe Output)
```

## 🛠️ File Changes

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/utils/security.go` | Modify | Enhance `ValidatePath` with `GetRepoRoot()` integration and `EvalSymlinks`. |
| `internal/utils/security.go` | Add | Implement `SanitizeFTS` helper. |
| `internal/store/store.go` | Modify | Apply `SanitizeFTS` to all FTS MATCH queries. |
| `internal/mcp/handlers.go` | Modify | Enforce row limits (e.g., 500 max) in all handlers returning lists (`handleSearch`, etc.). |
| `LICENSE` | Add | Create MIT License file. |

## 📜 Interfaces / Contracts

```go
// internal/utils/security.go

// SanitizeFTS sanitizes a raw search string for safe use in SQLite FTS5 MATCH expressions.
func SanitizeFTS(query string) string

// ValidatePath ensures the path is strictly within the repository root boundary.
func ValidatePath(baseDir string, targetPath string) (string, error)
```
