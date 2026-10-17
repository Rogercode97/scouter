# Organic Driven Development

Working documents for this repository's default workflow: one file per work unit, planned,
specified, executed and recorded in the same place.

| Directory | Holds | Lifecycle |
| :--- | :--- | :--- |
| `tasks/` | One document per active work unit: numbered specs (`S#`), the task list, and the evidence log. | Written before the first source change; it is the record of what was done and what proved it. |
| `specs/` | Capability specifications: behaviour that exists today. | Living. Updated as the behaviour changes. |
| `archive/` | Change records that are closed and no longer drive work. | Historical. Read `archive/README.md` before trusting any verification claim in there. |

`AGENTS.md` holds the repository's technical mandates and is read by every agent session; this
directory holds the per-unit work. A feature document under `tasks/` is the specification a
subagent reads, so it must stand on its own: the user's exact wording in `## Specs`, one line per
task, and the evidence in `## Log`.

## Provenance of `specs/` and `archive/`

Both were recovered from the retired `openspec/` tree, which was deleted from the default branch
and survived only in commit `e7a6b90` ("v1.0.0 release (Hakai Edition)", 2026-05-16) and on two
branches:
- `odd/specs/`: 9 capability specifications.
- `odd/archive/changes/`: 15 historical change records (70 files) carrying verification notices.
