# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| v1.x    | ✅ Active Development |
| < v1.0  | ❌ Not Supported    |

## Reporting a Vulnerability

**GitHub private vulnerability reporting is enabled for this repository.** A report filed there is visible only to the maintainers, so it is safe to include exploit details:

**→ [Report a vulnerability](https://github.com/Rogercode97/scouter/security/advisories/new)**

If that path is unavailable to you, email the maintainer at `rogercarp97@gmail.com` with the same details. Please do not use a public issue or a public discussion for a vulnerability report.

### What to include

1. The affected version or commit.
2. A description of the issue and its impact.
3. Steps to reproduce, ideally a minimal repository or an exact command sequence.
4. Any proof-of-concept output you already produced.

### Process

1. **Do not open a public issue.** A public report hands an attacker a head start.
2. We acknowledge receipt and provide an estimated timeframe for a fix.
3. Once a fix is verified, we coordinate a public disclosure with you.

## Scope

The security-relevant surfaces of this project are:

- The Staging Ledger and the mutation path (`internal/engine/ledger.go`, `internal/engine/evolution_engine.go`, `internal/engine/apply/`).
- The command execution guard (`internal/utils/safe_exec.go`), the allow-listed path for running external tools.
- The MCP server surface (`internal/mcp/`) and the file paths it can reach.
- The SQLite persistence layer (`internal/store/`) and the path validation it depends on.

Accuracy questions about the analysis engines — missed call sites, wrong blast radius, incomplete symbol coverage — belong in a normal issue, not in this channel. Their limits are documented in [docs/architecture/analysis-limits.md](docs/architecture/analysis-limits.md).

---
*Security and integrity are core tenets of Scouter's development.*
