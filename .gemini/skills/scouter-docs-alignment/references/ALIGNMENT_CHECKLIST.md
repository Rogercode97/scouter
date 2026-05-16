# Documentation Alignment Checklist

Before completing any work unit that affects Scouter's behavior, verify the following:

## 📐 Architectural Context
- [ ] Does `docs/CONTEXT.md` correctly reflect the current state of the `TruthEngine`?
- [ ] Are the "Architectural Pillars" still accurate after this change?
- [ ] Does `docs/ARCHITECTURE.md` need a diagram update or a new section?

## 🚀 Command Mastery
- [ ] Are all `scouter` command examples in the README.md and `scouter-helper` skill still functional?
- [ ] Have you tested the specific command flags mentioned in the docs?
- [ ] Is there a new command that needs to be documented?

## 🧠 Historical Wisdom
- [ ] Does `docs/SABIDURIA.md` need a new entry for this architectural decision?
- [ ] Are there any outdated ADRs (Architecture Decision Records) that should be marked as "Superceded"?

## 🛠️ MCP Tooling
- [ ] Are the MCP tool descriptions in `internal/mcp` aligned with the `SKILL.md` files?
- [ ] Do the agent instructions in `AGENTS.md` accurately describe how to use the new logic?
