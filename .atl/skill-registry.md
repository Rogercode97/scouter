# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| Go 1.24, Backend | go-dominion | ~/.gemini/skills/go-dominion/SKILL.md |
| Security, JWT | security-dominion | ~/.gemini/skills/security-dominion/SKILL.md |
| MCP, Deployment | deployment-sovereign | ~/.gemini/skills/deployment-sovereign/SKILL.md |
| Scouter, Analysis | scouter-dominion | ~/.gemini/skills/scouter-dominion/SKILL.md |
| Architect, Refactor | scouter-oracle | .gemini/agents/scouter-oracle.md |
| Purge, Refactor | scouter-refactor-shinigami | .gemini/agents/scouter-refactor-shinigami.md |
| Web, UI | web-dominion | ~/.gemini/skills/web-dominion/SKILL.md |

## Compact Rules

### go-dominion
- Use Go 1.24+ standards: `context.Context` is mandatory in all functions involving I/O or long tasks.
- No global mutable state. Use dependency injection (Interfaces/Structs).
- Errors: Wrap with `%w`, handle sentinel errors. Use `t.Context()` in tests for resource cleanup.
- MCP Server: Use `mu sync.Mutex` for `os.Stdout` to prevent framing corruption. Avoid `fmt.Fprint` on stdout.
- Database: WAL mode mandatory. Use CTEs for recursive queries (faster). Compute metrics in SQL (synthesis).

### security-dominion
- Zero Logs: Never log secrets, keys, or PII. Use environment variables.
- Validation: All external inputs (flags, JSON) MUST be validated with `validator/v10` or Zod.
- SQL Safety: Use `sanitizeFTS` for virtual tables to prevent injection.

### scouter-dominion
- MANDATORY: Use `scouter_search` -> `scouter_callers` -> `scouter_read` for all code investigation.
- IMPACT FIRST: Execute `scouter_impact` before proposing structural changes.
- SIGNAL: Use `scouter_pure_signal` via MCP to purify large noisy outputs using RTK.
- PREDICTIVE: Use `scouter_predict` to identify target tests post-implementation.

### scouter-oracle
- OMNISCIENCE: Map the entire dependency tree before suggesting refactors.
- ARCHITECTURE: Enforce Hexagonal decoupling and Domain Sovereignty.
- PROACTIVE: Identify symbols with high risk (centrality + fragility) using `scouter critical`.

### scouter-refactor-shinigami
- PURGE: Detect and eliminate dead code or redundant abstractions.
- DECOUPLING: Inject `context.Context` into legacy signatures to enable observability.
- STANDARDS: Align all refactored code with Go 1.24+ idioms.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| GEMINI.md | GEMINI.md | Main project mandate (Wave 8.9) |
| SABIDURIA.md | SABIDURIA.md | Architectural Wisdom and Battle Log |
| .atl/skill-registry.md | .atl/skill-registry.md | This registry |

Read the convention files listed above for project-specific patterns and rules.
