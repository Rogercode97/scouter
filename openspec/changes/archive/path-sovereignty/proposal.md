# 🏛️ PROPOSAL: PATH SOVEREIGNTY (WAVE 8.9)

## 🎯 INTENT
- **RCA**: Hardcoded paths (`/data/data/com.termux/files/usr/tmp/`) in `internal/utils/security.go` create environment leakage, break portability (Linux/MacOS), and violate Zero Trust principles. Supreme Judgment flagged this as DANGEROUS (environment leakage and potential TOCTOU vulnerabilities).
- **Value**: Establishes OS-agnostic path validation leveraging `os.UserHomeDir()` and `os.TempDir()`. Introduces a strict Deny-by-Default policy for sensitive directories to guarantee absolute filesystem sovereignty.

## 🛡️ SCOPE BOUNDARIES

### IN SCOPE
- Refactoring of `ValidatePath` (and related path validation logic) in `internal/utils/security.go`.
- Dynamic path resolution using standard `os` package functions.
- Implementation of a Deny-by-Default policy blocking access to `.git`, `.ssh`, `.env`, and `.scouter`.
- Updating corresponding unit tests to validate cross-platform scenarios and security policies.

### OUT OF SCOPE
- Refactoring of unrelated utility functions in `internal/utils/`.
- Changes to I/O write operations outside the scope of path validation.
- Implementation of OS-level jails or chroot environments.

## ⚔️ CAPABILITIES (CONTRACT)

| Capability | Type | Description |
| :--- | :--- | :--- |
| `validate-path-dynamic-resolution` | Modified | Replaces hardcoded Termux paths with dynamic resolution via `os.UserHomeDir()` and `os.TempDir()`. |
| `deny-by-default-sensitive-dirs` | New | Rejects any path intersecting with `.git`, `.ssh`, `.env`, or `.scouter` even if within allowed boundaries. |
| `cross-platform-path-validation` | Modified | Ensures path validation is deterministic across Android (Termux), Linux, and MacOS. |

## 🗺️ AFFECTED AREAS
- `internal/utils/security.go`
- `internal/utils/utils_test.go` (or associated test files for security utilities)

## 🔙 ROLLBACK PLAN
- **Action**: `git revert` the commit implementing Path Sovereignty.
- **Verification**: Execute `go test ./internal/utils/...` to confirm the restoration of the legacy Termux-bound validation logic.
