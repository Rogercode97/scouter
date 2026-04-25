# 🛡️ Specification: Path Sovereignty (Phase 2)

## 1. Intent
To enforce strict, unbypassable security boundaries around file system access within the Scouter workspace, ensuring no operations can escape the designated repository root (RepoRoot) or access sensitive/banned directories.

## 2. NEW Requirements

### Requirement: RepoRoot Detection
The system MUST detect the root of the repository by identifying the directory containing `go.mod` or `.git`. This directory SHALL act as the absolute boundary for all relative paths, unless explicitly allowed (e.g., temporary directories).

```gherkin
Feature: RepoRoot Boundary Detection
  As the Scouter security kernel
  I want to establish a secure filesystem boundary
  So that operations are contained within the project space

  Scenario: Successful detection via go.mod
    Given a project structure containing a "go.mod" file in "/app"
    When the system initializes the path sovereign
    Then it MUST set the RepoRoot to "/app"

  Scenario: Successful detection via .git
    Given a project structure containing a ".git" directory in "/workspace"
    And no "go.mod" file exists
    When the system initializes the path sovereign
    Then it MUST set the RepoRoot to "/workspace"
    
  Scenario: Failure to detect root
    Given a project structure with neither "go.mod" nor ".git"
    When the system initializes the path sovereign
    Then it MUST return an error indicating the RepoRoot could not be determined
```

### Requirement: Relative Path Validation
The system MUST approve relative paths that resolve strictly within the RepoRoot. It MUST NOT approve any absolute paths, except for explicitly whitelisted system directories like `os.TempDir()`.

```gherkin
Feature: Path Resolution and Approval
  As the Scouter security kernel
  I want to validate incoming file paths
  So that only safe, contained paths are processed

  Scenario: Valid relative path approval
    Given the RepoRoot is "/workspace"
    And a requested path is "src/main.go"
    When the path is evaluated for sovereignty
    Then the system MUST approve the path
    And resolve it to "/workspace/src/main.go"

  Scenario: Absolute path rejection
    Given the RepoRoot is "/workspace"
    And a requested path is "/etc/config.yaml"
    When the path is evaluated for sovereignty
    Then the system MUST NOT approve the path
    And return a "Security Violation: Absolute Paths Prohibited" error

  Scenario: Temporary directory approval
    Given the OS temporary directory is "/tmp"
    And a requested path is "/tmp/scouter_cache/data.json"
    When the path is evaluated for sovereignty
    Then the system MUST approve the path
```

### Requirement: Traversal and Blacklist Prevention
The system MUST NOT allow path traversal attacks (`../`) that escape the RepoRoot. Additionally, it MUST strictly block access to a predefined purity blacklist, including but not limited to `.git`, `.ssh`, `.env`, and `node_modules`.

```gherkin
Feature: Path Traversal and Blacklist Blocking
  As the Scouter security kernel
  I want to block malicious or noisy directory access
  So that sensitive data is protected and Ki budget is preserved

  Scenario: Path traversal attempt escaping RepoRoot
    Given the RepoRoot is "/workspace/project"
    And a requested path is "../../etc/passwd"
    When the path is evaluated for sovereignty
    Then the system MUST NOT approve the path
    And return a "Security Violation: Path Escapes RepoRoot" error

  Scenario: Accessing blacklisted directories
    Given the RepoRoot is "/workspace"
    And a requested path is ".git/config"
    When the path is evaluated for sovereignty
    Then the system MUST NOT approve the path
    And return a "Security Violation: Access to Blacklisted Path" error

  Scenario: Accessing blacklisted files
    Given the RepoRoot is "/workspace"
    And a requested path is ".env"
    When the path is evaluated for sovereignty
    Then the system MUST NOT approve the path
    And return a "Security Violation: Access to Blacklisted Path" error
```

### Requirement: Symlink Resolution
The system SHALL resolve symlinks before validating the final path. If a symlink resolves to a location outside the RepoRoot (and not in an explicitly whitelisted directory), it MUST be blocked.

```gherkin
Feature: Symlink Sovereignty
  As the Scouter security kernel
  I want to evaluate the real destination of symlinks
  So that boundaries cannot be bypassed via links

  Scenario: Symlink resolving inside RepoRoot
    Given the RepoRoot is "/workspace"
    And a symlink "docs/latest" points to "/workspace/docs/v2"
    When the path "docs/latest/index.md" is evaluated for sovereignty
    Then the system MUST approve the path

  Scenario: Symlink resolving outside RepoRoot
    Given the RepoRoot is "/workspace"
    And a symlink "src/external" points to "/var/www/shared"
    When the path "src/external/data.json" is evaluated for sovereignty
    Then the system MUST NOT approve the path
    And return a "Security Violation: Symlink Escapes RepoRoot" error
```