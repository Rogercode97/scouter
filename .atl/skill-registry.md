# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| Go 1.24, Backend | go-dominion | ~/.gemini/skills/go-dominion/SKILL.md |
| Security, JWT | security-dominion | ~/.gemini/skills/security-dominion/SKILL.md |
| MCP, Deployment | deployment-sovereign | ~/.gemini/skills/deployment-sovereign/SKILL.md |
| Scouter, Analysis | scouter-refactor-shinigami | .gemini/agents/scouter-refactor-shinigami.md |

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

### scouter-mandate (Recursive Sovereignty)
- **MANDATORY**: Before reading or writing code, sub-agents MUST use `scouter_index` to map symbols and `scouter_search` for semantic lookup.
- **IMPACT FIRST**: Any change proposal MUST include a blast radius analysis using `scouter_impact`.
- **PREDICTIVE VALIDATION**: After implementation, use `scouter_predict` to verify which tests must be executed.
- **ZERO BLINDNESS**: Do not guess file paths or symbol locations; verify them in the Call Graph.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| GEMINI.md | GEMINI.md | Main project mandate and conventions |
| SABIDURIA.md | SABIDURIA.md | Architectural Wisdom and Oracle Log |

Read the convention files listed above for project-specific patterns and rules.
