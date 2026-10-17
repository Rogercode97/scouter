# Organic Driven Development

Working documents for this repository's default workflow: one file per work unit, planned,
specified, executed and recorded in the same place.

| Directory | Holds | Lifecycle |
| :--- | :--- | :--- |
| `tasks/` | One document per active work unit: numbered specs (`S#`), the task list, and the evidence log. | Written before the first source change; it is the record of what was done and what proved it. |
| `specs/` | Capability specifications: behaviour that exists today. | Living. Updated as the behaviour changes. |

`AGENTS.md` holds the repository's technical mandates and is read by every agent session; this
directory holds the per-unit work. A feature document under `tasks/` is the specification a
subagent reads, so it must stand on its own: the user's exact wording in `## Specs`, one line per
task, and the evidence in `## Log`.

## Provenance of `specs/`

`specs/` was recovered from the retired `openspec/specs/` tree, which was deleted from the default
branch and survived only in commit `e7a6b90` ("v1.0.0 release (Hakai Edition)", 2026-05-16) and
on two branches. The files are unchanged from there.

They are descriptions of behaviour, so they carry no verification claim of their own — but they
were not re-checked against the code either. Where a capability spec and the code disagree, the
code is what ships.

The archived change records from the same tree are recovered separately. They are a different
proposition: 70 files, and a reliability question the specs do not have.
