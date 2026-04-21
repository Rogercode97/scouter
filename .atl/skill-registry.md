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

## Compact Rules

### go-dominion
- Use Go 1.24+ standards: `context.Context` is mandatory in all functions involving I/O or long tasks.
- No global mutable state. Use dependency injection (Interfaces/Structs).
- Errors must be wrapped (`fmt.Errorf("...: %w", err)`) and handle sentinel errors if applicable.
- Testing: Native `go test` with `iter.Seq` where applicable for high-performance iteration.
- FTS5: Manual virtual table sync required after schema changes in content tables.

### security-dominion
- Never log secrets or include them in code. Use environment variables.
- All external inputs (flags, JSON) MUST be validated with `validator/v10`.
- Use `sanitizeFTS` for SQL queries to prevent injection in virtual tables.

### scouter-dominion
- MANDATORY: Use `scouter_search` -> `scouter_callers` -> `scouter_read` for all code investigation. No `grep` or `read_file` allowed.
- IMPACT FIRST: Always execute `scouter_impact` before proposing structural changes.
- PREDICTIVE VALIDATION: Use `scouter_predict` to identify target tests after implementation.
- ZERO BLINDNESS: Trust the AST and LSP data over filename guessing.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| GEMINI.md | GEMINI.md | Main project mandate and conventions |
| SABIDURIA.md | SABIDURIA.md | Architectural Wisdom and Oracle Log |

Read the convention files listed above for project-specific patterns and rules.
