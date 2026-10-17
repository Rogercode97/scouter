# Feature: store-schema-migration

Locator: `odd/tasks/store-schema-migration.md` — Engram mirror topic `odd/store-schema-migration/tasks` (project `scouter`).
Branch: `fix/store-schema-migration`.

## Specs

S1 — An existing SQLite DB whose schema predates the current one MUST be converged to the current schema by `NewStore`, without losing existing rows. Evidence: today `NewStore` returns `err = <nil>` on such a DB and every subsequent operation fails — `table symbols has no column named package_path`, `no such column: symbols.package_path`, `table file_index has no column named freshness`.

S2 — Convergence MUST be idempotent. Opening an already-converged DB MUST apply no schema change, MUST NOT error, and MUST NOT duplicate columns.

S3 — `TestStore_Migration` MUST exercise a real old-schema DB. Today it builds its DSN as `fmt.Sprintf("%s?_pragma=foreign_keys(1)", dbPath)` without the `file:` prefix, so the old schema is written to a file literally named `test_scouter_migration.dbWrite?_pragma=foreign_keys(1)` while `NewStore` creates a brand-new DB at `test_scouter_migration.dbWrite`. The test asserts columns that a fresh schema always has, and has therefore never exercised migration.

S4 — `hasColumn` MUST reject a table name that is not part of the schema and a column name that is not a plain SQL identifier, and MUST NOT regress for the legitimate calls `hasColumn(..., "symbols", "name")`, `"symbols", "package_path"` and `"symbols", "receiver_type"`. Its current behavior for an unknown table is `(false, nil)`; S4 changes that contract to an error.

S5 — `CGO_ENABLED=0 go build ./...` MUST keep exiting 0.

S6 — Every `(table, column)` pair declared in `internal/store/schema.sql` MUST exist in a freshly created DB, and MUST be converged onto an older DB that lacks it. The convergence set is derived from `schema.sql`, never hand-maintained, so a new column cannot silently escape the ladder.

**User's original request, verbatim (L1)**: "continuamos?" → "lee memoria y handoff" → "activa rdd" → "usa codegraph" → fork elegido: "Construir la escalera de migración".

## Tasks

T1 — RED: fix the `TestStore_Migration` DSN so the old schema is real, and observe it fail — S3 — inline — '<commit pendiente>'
T2 — RED: convergence test from the real pre-`08bfeed` schema, with pre-existing rows and an idempotence check — S1, S2, S6 — inline — '<commit pendiente>'
T3 — Implement `internal/store/migrate.go`: parse `schema.sql` column definitions, `ALTER TABLE ADD COLUMN` for each missing column, wired into `NewStore` — S1, S2, S6 — inline — '<commit pendiente>'
T4 — Harden `hasColumn` with the schema-derived allow-list and an identifier check; update the unknown-table expectation — S4 — inline — '<commit pendiente>'
T5 — Parser rule tests + drift invariant with SQLite as the independent oracle — S6 — inline — '<commit pendiente>'
T6 — GREEN: store suite, build, vet, differential against `a1b839c` — S5 — inline — '<commit pendiente>'

## Log

L1 (user, verbatim): "continuamos?"

L2 (user, verbatim): "lee memoria y handoff"

L3 (user, verbatim): "activa rdd" — RDD enabled with `gentle-ai review mode enable --scope global`; clone-local can only disable.

L4 (user, verbatim): "usa codegraph" — unavailable: `codegraph binary was not found`. Fell back to `read`/`grep`/`find`.

L5 (evidence, probe on `b4047ea`): an old-schema DB opens with `NewStore` returning `err = <nil>`; then `SaveSymbol` fails with `table symbols has no column named package_path`, `GetSymbolsByPathPrefix` with `no such column: package_path`, `SearchSymbols` with `no such column: symbols.package_path`, `SaveFileIndex` with `table file_index has no column named freshness`.

L6 (evidence): `grep -rni "alter table\|add column\|ensurecolumn\|ensure_column" internal/` returns nothing. `migrate()` (`store.go:153`) only replays `CREATE TABLE IF NOT EXISTS` statements from `schema.sql`, which is a no-op on an existing table.

L7 (evidence): the false oracle confirmed by stat — with the existing test's DSN, `os.Stat(dbPath)` fails with "no such file or directory" while `os.Stat(dbPath + "?_pragma=foreign_keys(1)")` succeeds. Production DSNs at `store.go:97,105` do build `file:` correctly; the bug lives only in the tests.

L8 (decision): the ladder is derived from `schema.sql` at runtime rather than hand-written in Go. A hand-maintained ledger duplicates the schema and can drift; deriving it makes S6 true by construction. Rejected: `PRAGMA user_version` arithmetic alone, which cannot reconcile an unversioned DB whose column set is unknown.

L9 (decision): `hasColumn` on an unknown table returns an error instead of `(false, nil)`. Silent false hides a coding error in the one place we are about to depend on; the only caller that relied on the old behavior is the test at `store_test.go:378`.

L10 (evidence, RED observed before the fix, `a1b839c`): `TestStore_Migration` with the corrected DSN fails with `Column 'package_path' missing after migration` and `Column 'receiver_type' missing after migration`.

L11 (evidence, RED observed before the fix): the convergence test run from a scratch worktree at `a1b839c` with the identical test file fails with 8 missing columns — `symbols.{relevance,pagerank,churn_score,runtime_hits,ai_summary,metrics_json}`, `file_index.freshness`, `calls.body` — plus `GetFileIndex on legacy row: no such column: freshness`. The same file passes on the branch.

L12 (evidence, real releases): `git log -S` shows the missing columns entered `schema.sql` in `08bfeed` (2026-06-02, the sqlc migration) and `b99c49d` (2026-06-03). `internal/store/schema.sql` was created by `08bfeed`; before it the DDL lived inline in `store.go`. The test fixture uses that inline pre-`08bfeed` DDL verbatim, so the RED models a database a real release actually produced. The bug has shipped since 2026-06-02.

L13 (evidence, mutation testing — the tests have teeth, not just presence): truncating the parser to two columns per table makes `TestSchemaColumns_MatchFreshDatabase` fail naming parsed vs SQLite columns; removing the `hasColumn` guard from the ladder makes `TestSchemaConvergence_Idempotent` fail with `duplicate column name: path`. Both mutations reverted; the file was restored from a copy and the suite re-run green.

L14 (evidence, GREEN): `internal/store` ok with `-count=1`; `CGO_ENABLED=0 go build ./...` exit 0; `go vet ./internal/store/` exit 0.

L15 (evidence, differential, with the caveat stated): `internal/engine` fails 4 tests in the main checkout (`TestArchitecturalAuditIntegration`, `TestASTRuleIntegration`, `TestRippleEngine_StructuralIntegration`, `TestStructuralSearch`, all `sg` not in `PATH`) across three `-count=1` runs, and 5 in a scratch worktree at `a1b839c` — the same four plus `TestHealerEngine_AllFail`. That fifth difference is the harness location, not the change: it fails 3/3 in the scratch worktree and 0/3 in the main checkout, and this diff does not touch `internal/engine`. A separate one-off `TestExecuteStderr` failure in an earlier differential run did not reproduce in any of three subsequent clean runs. No package other than `internal/store` changes.

L16 (decision): the diff does not attempt to add `--rebuild` or a schema-version guard. Convergence is the requested mechanism; a user-visible escape hatch is a separate unit if it is ever wanted.
