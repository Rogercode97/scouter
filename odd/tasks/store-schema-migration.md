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

T1 — RED: fix the `TestStore_Migration` DSN so the old schema is real, and observe it fail — S3 — inline — f3fd0cd
T2 — RED: convergence test from the real pre-`08bfeed` schema, with pre-existing rows and an idempotence check — S1, S2, S6 — inline — f3fd0cd
T3 — Implement `internal/store/migrate.go`: parse `schema.sql` column definitions, `ALTER TABLE ADD COLUMN` for each missing column, wired into `NewStore` — S1, S2, S6 — inline — f3fd0cd
T4 — Harden `hasColumn` with the schema-derived allow-list and an identifier check; update the unknown-table expectation — S4 — inline — f3fd0cd
T5 — Parser rule tests + drift invariant with SQLite as the independent oracle — S6 — inline — f3fd0cd
T6 — GREEN: store suite, build, vet, differential against `a1b839c` — S5 — inline — f3fd0cd

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

L17 (decision, slice size): the unit is 667 authored changed lines in one commit, past the ~400-line planning heuristic. The overage is the test surface, not the mechanism: `migrate.go` is 227 lines, `migrate_test.go` is 321, and the faithful pre-`08bfeed` fixture plus the SQLite-oracle invariant are what make S1 and S6 falsifiable. Splitting tests from the code they falsify would leave a commit that asserts nothing, so the slice stays whole and the boundary is recorded here.

L18 (evidence, independent verification of `f3fd0cd` by a separate verifier, all probes run in scratch copies): every spec MET. S1 `TestSchemaConvergence_OldDatabase` PASS with the legacy row intact; S2 `TestSchemaConvergence_Idempotent` PASS; S3 `TestStore_Migration` PASS, and the verifier confirmed the false oracle independently by running the same test on `a1b839c`, where it still passes without any migration having happened; S4 PASS with no caller outside `migrate.go` and the tests; S5 build exit 0; S6 `TestSchemaColumns_MatchFreshDatabase` PASS. Its falsification attempts also held: adding `probe_col TEXT DEFAULT 'x' NOT NULL` to a scratch `schema.sql` was picked up with no Go change; a column that cannot be added (`bad_col INTEGER NOT NULL` with no default) fails `NewStore` with `schema convergence failed: adding symbols.bad_col: ... Cannot add a NOT NULL column with default value NULL`, and after that failure the database is rolled back cleanly — `bad_col` absent, the legacy row intact (`hash-legacy`, `mtime=42`), the `flows` table not created and the database still queryable. The fresh path issues zero ALTERs. It also searched the repository for other DSNs built as `path + "?_pragma=..."` without `file:` and found none.

L19 (evidence, per-open cost — measured by the independent verifier, not inferred): convergence added `+2.71 ms` to every store open on a converged database (`3.82 ms` against `1.09 ms` at `a1b839c`), because the ladder resolved one column per `pragma_table_info` query, about sixty of them. `scouter` opens the store once per CLI invocation, so that was a per-command tax invisible to the functional tests. Fixed in `4c93d7c` by reading each table's column set once and diffing in memory: `+0.48 ms` (`1.57 ms` per open), a 5.6x reduction of the added cost. Committed benchmarks `BenchmarkNewStoreOpen` and `BenchmarkNewStoreOpenOLDDatabase` keep it visible. The mutation that removes the existing-column guard still fails `TestSchemaConvergence_OldDatabase` and `TestSchemaConvergence_Idempotent` with `duplicate column name: path`, so the refactor did not trade correctness for speed.

L20 (open, unverified): the end-to-end CLI impact of the remaining `+0.48 ms` is NOT measured. There is no indexed database on this machine (`~/.config/scouter/` does not exist), and creating one writes outside the project root, so the comparison against a real `scouter search` invocation was not run. `+0.48 ms` is 44% of the base open cost, which is why it was worth removing, but that base open is itself a fixed millisecond of schema replay. Do not describe this as negligible, and do not describe it as harmful: it is unmeasured. A `PRAGMA user_version` stamp would remove the cost entirely by skipping the check, at the price of a second source of truth and a database that never self-heals from a manual schema change; not taken.

L21 (evidence, delta verification of `4c93d7c` by the same independent verifier, scratch copies only): clean, no defects. Every spec re-confirmed on the new tree — S1, S2, S3, S4, S6 all PASS with the same probes as the first round, so the comparison is apples to apples, and the DSN search again found no `path + "?_pragma=..."` without `file:`. The falsification it established still holds after the refactor: an unaddable column fails `NewStore` and rolls back completely, with the database file size unchanged (`53248 -> 53248`), the legacy row intact and the `flows` table absent. New-path probes confirmed the batching changes nothing observable: `tableColumns` agrees with the old per-column conclusion on both a fresh and an old database, membership and counts alike; an unknown table still errors; no phantom or missing column. Its own measurement: `1.61 ms` per open on `4c93d7c` against `4.20 ms` on `f3fd0cd`, so the remaining added cost is `~0.52 ms` and the commit's `+0.48 ms` is a fair characterization within run-to-run variance.

L22 (decision, delivery): the unit was published only after the delta came back clean, which is what the user chose. The unmeasured end-to-end CLI impact in L20 travelled with the change rather than being quietly dropped.

L23 (evidence, L20 closed — measured 2026-10-25 on the reference machine: Go 1.27.1-X:nodwarf5 linux/amd64, AMD Ryzen 5 5600GT, 12 procs, ripgrep 15.2.0 at `/usr/bin/rg`, same machine as L9): isolated scratch `SCOUTER_CONFIG=/tmp/scouter-l20/config.toml` (`db_path=/tmp/scouter-l20/scouter.db`, transient, removed after the run), corpus = the repository itself. `scouter index .` exits 0 in 13,379 ms, DB 15,441,920 bytes — same order as the L13 baseline (12.6 s, ~14.9 MB). Cold `scouter search "Ledger"` exits 0 in 48/46/46/48/46 ms across 5 runs (~47 ms wall). `BenchmarkNewStoreOpen` (converged DB, `benchtime=10x`) reports 0.637 ms/op on this machine; the whole converged open is therefore ~1.4% of a 47 ms search wall, and the ladder is only a fraction of that open (the rest is SQLite open plus the schema replay that predates it). The verifier's `+0.48 ms` delta is smaller than the observed run-to-run search jitter (±1 ms), and the index command pays one open against 13.4 s (~0.005%). Single-open-per-invocation confirmed by reading the path: `search` calls `openDB` → one `store.NewStore`, and the telemetry tracker stays lazy on this path (`WarmUp` runs only in the proxy pipeline). Verdict: the remaining ladder cost is real but one order below the noise floor of the commands that pay it — negligible in practice, measured rather than asserted. Sample-size caveat: benchmarks at `benchtime=10x`, search at N=5; fractions are stable to that precision, absolutes are not portable across machines (this machine opens in 0.64 ms where the verifier's measured 1.57 ms).
