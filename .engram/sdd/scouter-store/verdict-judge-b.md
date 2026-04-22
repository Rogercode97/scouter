# Supreme Judgment: Judge-B Report (Persistence Layer)

## ⚖️ Score: 6.5 / 10.0 (HAKAI RATING: REJECTED)

## 🛡️ 1. SQL Integrity (Pass)
**Status:** Highly Resilient
- **Analysis:** Zero SQL injection vulnerabilities detected. All queries strictly use parameterized arguments (`?`). 
- **Sanitization:** `sanitizeFTS(q)` correctly escapes double quotes and handles wildcards safely for FTS5 queries. 
- **Verdict:** Solid. No brittle query building found.

## 🗄️ 2. SQLite Locking (Fail - Concurrency Flaw)
**Status:** Brittle under Load
- **Analysis:** `New()` configures `_pragma=journal_mode(WAL)` and `_pragma=busy_timeout(5000)`. However, it completely fails to limit the `database/sql` connection pool. 
- **Flaw:** Go's `sql.DB` will open multiple connections. In SQLite WAL mode, multiple connections attempting concurrent writes will cause unresolvable `SQLITE_BUSY` ("database is locked") errors despite the busy timeout, because the driver allows uncontrolled concurrency that SQLite cannot queue properly across multiple connections.
- **Mandatory Fix:** Must inject `db.SetMaxOpenConns(1)` immediately after `sql.Open` to serialize write transactions safely within Go.

## 🔎 3. FTS5 Efficiency & Migration (Critical Fail - Corrupt Schema Evolution)
**Status:** Broken Migration
- **Analysis:** The virtual FTS5 index is implemented as an external content table (`content='symbols'`). This is efficient. However, the migration logic is fundamentally flawed.
- **Flaw:** The `migrate()` function checks for the `doc` column using `hasColumn()`. If missing, it uses `ALTER TABLE symbols ADD COLUMN doc TEXT` and *recreates the triggers*. IT DOES NOT RECREATE `symbols_fts`! 
- **Consequence:** `CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts` does nothing if the old index already exists. The old index lacks the `doc` column. When the newly created triggers fire, SQLite will crash with "table symbols_fts has no column named doc".
- **Mandatory Fix:** If the `doc` column is added, the migration MUST drop the old `symbols_fts` table and recreate it, or perform a full table rebuild via `INSERT INTO symbols_fts(symbols_fts) VALUES('rebuild')` after structurally replacing it.

## 💀 Final Verdict
**HAKAI.** The FTS5 migration bug guarantees a crash on schema evolution, and missing connection limits violate Wave 8.9 concurrency standards. Reject and demand immediate remediation.
