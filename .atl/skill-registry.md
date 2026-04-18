# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

See `_shared/skill-resolver.md` for the full resolution protocol.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| Supreme protocol for Go 1.24+ Hexagonal Architecture. Enforces Domain Sovereignty, Strict Validation, and Context-First. | go-divine | /data/data/com.termux/files/home/hakaishin-vault/skills/go-divine/SKILL.md |
| Supreme protocol for Model Context Protocol (MCP) servers. Enforces tool security via Zod, resource management, and transport isolation. | mcp-sovereign | /data/data/com.termux/files/home/hakaishin-vault/skills/mcp-sovereign/SKILL.md |
| High-density TDD protocol. Enforces Vitest v3+, Branch Coverage, and Auto-Update Thresholds. | tdd-discipline | /data/data/com.termux/files/home/hakaishin-vault/skills/tdd-discipline/SKILL.md |
| Supreme protocol for systematic debugging. Enforces 5 Whys, Fault Tree Analysis (FTA), and Bisection. | root-cause-shinigami | /data/data/com.termux/files/home/hakaishin-vault/skills/root-cause-shinigami/SKILL.md |
| When creating a GitHub issue, reporting a bug, or requesting a feature. | issue-creation | /data/data/com.termux/files/home/.gemini/skills/issue-creation/SKILL.md |
| When writing Go tests, using teatest, or adding test coverage. | go-testing | /data/data/com.termux/files/home/.gemini/skills/go-testing/SKILL.md |
| When creating a pull request, opening a PR, or preparing changes for review. | branch-pr | /data/data/com.termux/files/home/.gemini/skills/branch-pr/SKILL.md |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.

### go-divine
- Domain Sovereignty: Logic MUST be agnostic to frameworks/DBs using Interfaces (Ports).
- Glasswall Validation: External structs MUST have `validate` tags and be verified at entry.
- Context-First: All I/O and long-running functions MUST accept `context.Context` as the first arg.
- Go 1.24+ Testing: Use `t.Context()` to ensure goroutine cancellation and resource cleanup.
- Error Handling: Use explicit errors; NEVER use `panic` for flow control.

### mcp-sovereign
- Zod-Strict Tools: Every tool MUST have an `inputSchema` validated strictly with Zod.
- Resource Governance: Use `registerResource` with consistent URI schemes (`protocol://path`).
- Transport Isolation: Logic MUST be independent of the transport layer (stdio, HTTP, etc.).
- Structured Errors: Return errors in `TextContent` for agent interpretation; no stack leaks.

### tdd-discipline
- Branch Supremacy: Target **Branch Coverage >= 90%** for all logical paths.
- Auto-Update Thresholds: Use `autoUpdate: true` in Vitest to lock coverage improvements.
- Feedback Velocity: Red-Green cycle MUST take <2s (use HMR/Watch Mode).
- In-Source Purity: Use `import.meta.vitest` for co-locating critical utility tests.

### root-cause-shinigami
- 5 Whys Rigor: Dig 5 levels deep until reaching the process or design root cause.
- Fault Tree Analysis (FTA): Map dependencies to find the Single Point of Failure.
- Binary Bisection: Systematically narrow the search space (commits, inputs, logic).
- Deterministic Proof: A failing regression test is MANDATORY before applying any fix.
- Documentation: Always record RCA as "What/Why/Where/Learned" in Engram.

### go-testing
- Use standard `go test` with `-v` and `-cover`.
- Follow Go conventions (test files `_test.go` in the same package).
- Prefer `teatest` for Bubbletea TUI testing.

### issue-creation
- Follow the issue-first enforcement system. Create issues before implementation.

### branch-pr
- Follow the issue-first enforcement system. Link PRs to issues.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| CLAUDE.md | internal/engine/CLAUDE.md | Architecture, conventions, dev commands. |
| CONTRIBUTING.md | internal/engine/CONTRIBUTING.md | Contribution guidelines, conventional commits. |
| README.md | README.md | High-level overview and V1.1 Hakaishin features. |
| justfile | justfile | Common tasks (run, build, test, fmt, sync). |
| Makefile | Makefile | Build and test commands. |

Read the convention files listed above for project-specific patterns and rules.
