# 🛡️ CHANGE PROPOSAL: PUBLIC RELEASE SECURITY HARDENING

## 🎯 INTENT (RCA & VALUE)
Transform Scouter into a secure, public-ready project by implementing industry-standard security protocols and "Divine Class" architectural shielding. The goal is to eliminate potential Path Traversal, SQL Injection (FTS5), and DoS via unbounded queries, ensuring local execution remains strictly bound to user intent.

## ⚖️ SCOPE DEFINITION

### IN SCOPE (KINETIC STRIKE)
- **Path Sovereignty**: Refine `ValidatePath` (`internal/utils/security.go`) to enforce a strict "Project Jail" using `filepath.EvalSymlinks` and `filepath.Abs`.
- **SQL Armor**: Audit and harden `internal/store/store.go` to ensure 100% parameterization, specifically for FTS5 queries.
- **Secret Protection**: Update `.gitignore` and establish a protocol for history scanning (Gitleaks/Trufflehog).
- **System Integrity**: Implement `maxDepth` hard-caps for Recursive CTEs to prevent DoS.
- **Legal Sovereignty**: Add an MIT `LICENSE` file and standard headers.

### OUT OF SCOPE (HAKAI)
- Implementing a full-blown Auth/RBAC system (Scouter is a local tool).
- Remote telemetry or cloud integration.

## 📦 CAPABILITIES AFFECTED
- `path-validation` (Modified)
- `fts-query-execution` (Modified)
- `recursive-dependency-resolution` (Modified)
- `secret-scanning-protocol` (New)
- `legal-sovereignty` (New)

## 📍 AFFECTED AREAS (BLAST RADIUS)
| Component | Path | Impacted Symbols |
| :--- | :--- | :--- |
| **Path Validator** | `internal/utils/security.go` | `ValidatePath` |
| **SQL Store** | `internal/store/store.go` | FTS5 Query Constructors, `NewStore` |
| **MCP Handlers** | `internal/mcp/handlers.go` | `Server.handleIndex`, `Server.handleRead` |
| **Project Root** | `.gitignore`, `LICENSE` | File Additions/Updates |

*Scouter Impact Analysis Confirmed: Path sovereignty changes directly impact file read and index handlers.*

## 🔄 ROLLBACK PLAN
- Revert commits affecting `internal/utils/security.go` and `internal/store/store.go`.
- Drop FTS5 parameterization modifications.
- Remove `LICENSE` and `.gitignore` appended rules.
- Run `go test ./...` to verify state consistency.
