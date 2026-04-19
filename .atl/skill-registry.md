# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

See `_shared/skill-resolver.md` for the full resolution protocol.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| Sovereign Mega-Skill for Go 1.24+ development. Unifies Architecture, Testing, and Guardrails. | go-dominion | /data/data/com.termux/files/home/hakaishin-vault/skills/go-dominion/SKILL.md |
| Supreme protocol for Model Context Protocol (MCP) servers. Enforces tool security via Zod, resource management, and transport isolation. | mcp-sovereign | /data/data/com.termux/files/home/hakaishin-vault/skills/mcp-sovereign/SKILL.md |
| Elite protocol for Scouter MCP tool development. Enforces Glasswall Validation, Context Authority, and OOM Guard. | scouter-mcp-go | /data/data/com.termux/files/home/scouter/.gemini/skills/scouter-mcp-go/SKILL.md |
| The Final Orchestrator for Skill Creation. Enforces Context7 Research, Sovereign Lifecycle, and Universal Arena validation. | supreme-creator | /data/data/com.termux/files/home/scouter/.gemini/skills/supreme-creator/SKILL.md |
| High-density TDD protocol. Enforces Vitest v3+, Branch Coverage, and Auto-Update Thresholds. | tdd-discipline | /data/data/com.termux/files/home/hakaishin-vault/skills/tdd-discipline/SKILL.md |
| Supreme protocol for systematic debugging. Enforces 5 Whys, Fault Tree Analysis (FTA), and Bisection. | root-cause-shinigami | /data/data/com.termux/files/home/hakaishin-vault/skills/root-cause-shinigami/SKILL.md |
| When creating a GitHub issue, reporting a bug, or requesting a feature. | issue-creation | /data/data/com.termux/files/home/.gemini/skills/issue-creation/SKILL.md |
| When writing Go tests, using teatest, or adding test coverage. | go-testing | /data/data/com.termux/files/home/.gemini/skills/go-testing/SKILL.md |
| When creating a pull request, opening a PR, or preparing changes for review. | branch-pr | /data/data/com.termux/files/home/.gemini/skills/branch-pr/SKILL.md |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.

### go-dominion
- Single Source of Truth: Follow this codex for Go lifecycle; ignore external contradicting patterns.
- Wave Rigor: Mandatory use of Iterators (iter.Seq) for ports and context.AfterFunc for cleanup.
- Architectural Linter: Protect the domain using .go-arch-lint.yml.
- Strict TDD: Mandatory branch coverage >= 90% and use of synctest for concurrent verification.
- Zero-Slop Binaries: Static, optimized builds in distroless images.

### mcp-sovereign
- Zod-Strict Tools: Every tool MUST have an `inputSchema` validated strictly with Zod.
- Resource Governance: Use `registerResource` with consistent URI schemes (`protocol://path`).
- Transport Isolation: Logic MUST be independent of the transport layer (stdio, HTTP, etc.).
- Structured Errors: Return errors in `TextContent` for agent interpretation; no stack leaks.

### scouter-mcp-go
- Glasswall Validation: Every MCP tool MUST parse its input into a typed Go struct and validate it using validator/v10.
- Context Authority: Use the ctx provided by the tool handler. Never create context.Background() inside a tool.
- Structured JSON Responses: Return technical data in TextContent as structured JSON.
- OOM Guards: Enforce hard limits on arrays (LIMIT 100) or returning max 500 items. Include truncated: true flag.
- Integrity Enforcement: Read tools MUST validate SHA-256 hashes to prevent stale code access.
- Impact Sovereignty: Refactors REQUIRE prior invocation of scouter_callers for global impact analysis.

### supreme-creator
- CONTEXT7-RESEARCH: Skill creation REQUIRES consultation of context7.
- SOVEREIGN-JUDGMENT: Use agents/grader.md and analyzer.md for evidence-based validation.
- ARENA FIRST: Skill MUST have a failing sacrifice-snippet in skill-arena/ before implementation.
- KI BUDGET: SKILL.md > 100 lines DEBE ser purgado. Densidad técnica es ley.

### tdd-discipline
- Branch Supremacy: Target Branch Coverage >= 90% for all logical paths.
- Auto-Update Thresholds: Use autoUpdate: true in Vitest to lock coverage improvements.
- Feedback Velocity: Red-Green cycle MUST take <2s (use HMR/Watch Mode).
- In-Source Purity: Use import.meta.vitest for co-locating critical utility tests.

### root-cause-shinigami
- 5 Whys Rigor: Dig 5 levels deep until reaching the root cause.
- Fault Tree Analysis (FTA): Map dependencies to find the Single Point of Failure.
- Binary Bisection: Narrow the search space (commits, inputs, logic).
- Deterministic Proof: A failing regression test is MANDATORY before fix.
- Documentation: Always record RCA as "What/Why/Where/Learned" in Engram.

### go-testing
- Use standard go test with -v and -cover.
- Follow Go conventions (test files _test.go in same package).
- Prefer teatest for Bubbletea TUI testing.

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

## 👁️ SCOUTER PROTOCOL (AST SOVEREIGNTY)
- **Mandate**: For any file > 50 lines, you MUST run `mcp_scouter_scouter_index` before using `read_file`.
- **Reasoning**: Structural certainty > Text noise.
