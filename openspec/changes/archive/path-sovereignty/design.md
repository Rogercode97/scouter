# Technical Design: Path Sovereignty (Fase 2)

## 1. Technical Approach
Implement dynamic root discovery and strict path validation to ensure Scouter operates securely and portably across environments, eliminating Termux-specific hardcodes. All path resolutions will be anchored to the repository root or the OS temporary directory, fortified with a purity blacklist.

## 2. Architecture Decisions

### Decision 1: Dynamic Repo Root Discovery
*   **Choice**: Implement `GetRepoRoot()` to walk up the directory tree from `os.Getwd()` looking for `go.mod` or `.git`.
*   **Alternatives**: Rely on environment variables (e.g., `SCOUTER_ROOT`).
*   **Rationale**: Dynamic discovery ensures seamless operation without requiring manual user setup, adhering to the Zero Slop mandate for autonomous sovereignty.

### Decision 2: Strict Path Validation and Purity
*   **Choice**: Implement `ValidatePath(path string) (string, error)` enforcing:
    *   Rejection of absolute paths from user input.
    *   Resolution of symlinks using `filepath.EvalSymlinks`.
    *   Strict bounding to `GetRepoRoot()` or `os.TempDir()` using `filepath.Rel` and checking for `..` escapes.
    *   Purity Blacklist blocking: `.git`, `.ssh`, `.env`, `.scouter`, `node_modules`, `vendor`, `dist`, `build`, `.vscode`, `.idea`, `.DS_Store`.
*   **Alternatives**: Basic string prefix matching.
*   **Rationale**: `filepath.EvalSymlinks` combined with path bounding prevents directory traversal attacks (LFI/Path Traversal). The purity blacklist protects sensitive files and ignores noisy, generated directories, saving "Ki" (token budget).

### Decision 3: Total Portability (Eliminate Hardcodes)
*   **Choice**: Replace all hardcoded Termux paths (e.g., `/data/data/com.termux/files/home`) with `os.UserHomeDir()`, `os.TempDir()`, and `GetRepoRoot()`.
*   **Alternatives**: Keep hardcodes but add platform checks.
*   **Rationale**: Hardcodes violate portability. Standard library functions (`os` and `path/filepath`) guarantee cross-platform compatibility (Linux, macOS, Windows).

## 3. Data Flow

```ascii
[User Input Path] -> ValidatePath(path)
                          |
                          v
                 1. Is Absolute? -> [Reject]
                          |
                          v
                 2. EvalSymlinks(path)
                          |
                          v
                 3. GetRepoRoot() / os.TempDir()
                          |
                          v
                 4. Ensure Path is Bounded (No '..') -> [Reject if Escapes]
                          |
                          v
                 5. Check Purity Blacklist
                    (Blocks .env, .git, node_modules, etc.) -> [Reject if Match]
                          |
                          v
                   [Validated Absolute Path]
```

## 4. File Changes (Impact-Verified)

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/utils/utils.go` (or `path.go`) | Add | Introduce `GetRepoRoot()` and `ValidatePath()` functions with the purity blacklist. |
| `cmd/scouter/main.go` | Modify | Update entry points and flag parsing to use `ValidatePath()` on all input paths. |
| `internal/engine/*` | Modify | Replace Termux hardcodes with dynamic path discovery (`GetRepoRoot()`, `os.UserHomeDir()`). |

## 5. Interfaces / Contracts

```go
package utils

import "errors"

var (
	ErrPathAbsolute     = errors.New("absolute paths are not allowed")
	ErrPathEscape       = errors.New("path escapes repository root")
	ErrPathBlacklisted  = errors.New("path is in purity blacklist")
	ErrRootNotFound     = errors.New("repository root (go.mod or .git) not found")
)

// GetRepoRoot discovers the root directory of the current project dynamically.
func GetRepoRoot() (string, error)

// ValidatePath sanitizes, resolves, and bounds the given path to ensure sovereignty.
func ValidatePath(inputPath string) (string, error)
```
