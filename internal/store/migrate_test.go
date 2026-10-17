package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// oldSchemaSQL is the schema a real pre-08bfeed database has on disk, taken from
// the inline DDL that commit 08bfeed replaced with schema.sql. Such a database
// is missing file_index.freshness, symbols.{relevance, pagerank, churn_score,
// runtime_hits, ai_summary, metrics_json}, calls.body and the whole flows table,
// and nothing has ever added them: schema.sql only issues CREATE TABLE IF NOT
// EXISTS, which is a no-op on a table that already exists.
const oldSchemaSQL = `
CREATE TABLE file_index (path TEXT PRIMARY KEY, mtime INTEGER, hash TEXT, ast_json TEXT, project TEXT);
CREATE TABLE directory_hashes (path TEXT PRIMARY KEY, hash TEXT, mtime INTEGER);
CREATE TABLE symbols (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, package_path TEXT DEFAULT '', receiver_type TEXT DEFAULT '', signature TEXT DEFAULT '', doc TEXT, path TEXT, start_byte INTEGER, end_byte INTEGER, start_line INTEGER, start_col INTEGER, end_line INTEGER, structural_hash TEXT DEFAULT '', indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);
CREATE TABLE calls (id INTEGER PRIMARY KEY AUTOINCREMENT, caller_name TEXT NOT NULL, callee_name TEXT NOT NULL, path TEXT NOT NULL, line INTEGER NOT NULL, callee_path TEXT DEFAULT '', link_type TEXT DEFAULT 'call', indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);
CREATE TABLE dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version TEXT, type TEXT, project TEXT, direct INTEGER);
CREATE TABLE test_results (id INTEGER PRIMARY KEY AUTOINCREMENT, test_name TEXT NOT NULL, status TEXT NOT NULL, error_message TEXT, stack_trace TEXT, target_symbol TEXT, duration_ms INTEGER, project TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE violations (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_id TEXT, file_path TEXT, message TEXT, severity TEXT, start_line INTEGER, start_col INTEGER, text TEXT, FOREIGN KEY(file_path) REFERENCES file_index(path) ON DELETE CASCADE);
CREATE TABLE symbol_usage (id INTEGER PRIMARY KEY AUTOINCREMENT, symbol_id INTEGER NOT NULL, environment TEXT NOT NULL, last_used INTEGER NOT NULL, hit_count INTEGER NOT NULL, UNIQUE(symbol_id, environment), FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE);
`

type tableShape map[string]map[string]bool

func dumpShape(t *testing.T, db *sql.DB, label string) tableShape {
	t.Helper()
	shape := tableShape{}
	rows, err := db.Query(`SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '%_fts_%'
		ORDER BY name`)
	if err != nil {
		t.Fatalf("%s: query tables: %v", label, err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("%s: scan table: %v", label, err)
		}
		tables = append(tables, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: tables err: %v", label, err)
	}

	for _, table := range tables {
		cols, err := pragmaColumns(db, table)
		if err != nil {
			continue // virtual tables (fts5, vec0) do not answer pragma_table_info
		}
		set := map[string]bool{}
		for _, c := range cols {
			set[c] = true
		}
		shape[table] = set
	}
	return shape
}

func pragmaColumns(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, table string) ([]string, error) {
	rows, err := q.Query(fmt.Sprintf("SELECT name FROM pragma_table_info('%s')", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func writeOldDatabase(t testing.TB, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(oldSchemaSQL); err != nil {
		t.Fatalf("create old schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO file_index (path, mtime, hash, ast_json, project)
		VALUES ('/legacy.go', 42, 'hash-legacy', '{}', 'legacy-project')`); err != nil {
		t.Fatalf("seed file_index: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO symbols
		(name, type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree)
		VALUES ('Legacy', 'func', 'func Legacy()', '', '/legacy.go', 0, 10, 1, 1, 1, 'sh', 3)`); err != nil {
		t.Fatalf("seed symbols: %v", err)
	}
}

// S1/S6: an old DB must converge to the current schema, and the rows that were
// already in it must survive the convergence.
func TestSchemaConvergence_OldDatabase(t *testing.T) {
	ctx := context.Background()

	goldenPath := filepath.Join(t.TempDir(), "golden.db")
	golden, err := NewStore(ctx, goldenPath)
	if err != nil {
		t.Fatalf("golden NewStore: %v", err)
	}
	goldenShape := dumpShape(t, golden.(*storeImpl).dbWrite, "golden")
	golden.Close()

	oldPath := filepath.Join(t.TempDir(), "old.db")
	writeOldDatabase(t, oldPath)

	s, err := NewStore(ctx, oldPath)
	if err != nil {
		t.Fatalf("NewStore on old DB: %v", err)
	}
	defer s.Close()
	got := dumpShape(t, s.(*storeImpl).dbWrite, "converged")

	for table, want := range goldenShape {
		have, ok := got[table]
		if !ok {
			t.Errorf("table %q missing after convergence", table)
			continue
		}
		for col := range want {
			if !have[col] {
				t.Errorf("column %s.%s missing after convergence", table, col)
			}
		}
	}

	// Pre-existing rows survive.
	idx, err := s.GetFileIndex(ctx, "/legacy.go")
	if err != nil {
		t.Fatalf("GetFileIndex on legacy row: %v", err)
	}
	if idx == nil {
		t.Fatal("legacy file_index row was lost by convergence")
	}
	if idx.Hash != "hash-legacy" || idx.Mtime != 42 {
		t.Errorf("legacy row corrupted: hash=%q mtime=%d", idx.Hash, idx.Mtime)
	}

	// The operations that used to fail with "no such column" now work.
	if err := s.SaveSymbol(ctx, &Symbol{
		Name: "Fresh", Type: "func", PackagePath: "pkg", Signature: "func Fresh()",
		Doc: "", Path: "/legacy.go", StartByte: 0, EndByte: 5,
		StartLine: 2, StartCol: 1, EndLine: 2,
	}); err != nil {
		t.Errorf("SaveSymbol after convergence: %v", err)
	}
	if _, err := s.GetSymbolsByPathPrefix(ctx, "/legacy"); err != nil {
		t.Errorf("GetSymbolsByPathPrefix after convergence: %v", err)
	}
	if _, err := s.SearchSymbols(ctx, "Legacy", "", "", 10, 0); err != nil {
		t.Errorf("SearchSymbols after convergence: %v", err)
	}
	if err := s.SaveFileIndex(ctx, &FileIndex{
		Path: "/new.go", Mtime: 1, Hash: "h", AstJson: "{}", Project: "p",
	}); err != nil {
		t.Errorf("SaveFileIndex after convergence: %v", err)
	}
}

// S2: convergence is idempotent — a second open changes nothing and does not error.
func TestSchemaConvergence_Idempotent(t *testing.T) {
	ctx := context.Background()
	oldPath := filepath.Join(t.TempDir(), "old.db")
	writeOldDatabase(t, oldPath)

	first, err := NewStore(ctx, oldPath)
	if err != nil {
		t.Fatalf("first NewStore: %v", err)
	}
	firstShape := dumpShape(t, first.(*storeImpl).dbWrite, "first")
	first.Close()

	second, err := NewStore(ctx, oldPath)
	if err != nil {
		t.Fatalf("second NewStore: %v", err)
	}
	defer second.Close()
	secondShape := dumpShape(t, second.(*storeImpl).dbWrite, "second")

	if len(firstShape) != len(secondShape) {
		t.Fatalf("table count changed on reopen: %d -> %d", len(firstShape), len(secondShape))
	}
	for table, cols := range firstShape {
		reopened, ok := secondShape[table]
		if !ok {
			t.Errorf("table %q vanished on reopen", table)
			continue
		}
		if len(cols) != len(reopened) {
			t.Errorf("column count for %q changed on reopen: %d -> %d", table, len(cols), len(reopened))
		}
		for col := range cols {
			if !reopened[col] {
				t.Errorf("column %s.%s vanished on reopen", table, col)
			}
		}
	}
}

// S6: the parsed column set must equal what SQLite actually creates for the
// schema. SQLite is the independent oracle here: if the parser silently skips a
// column, the ladder skips it too, and nothing else would notice.
func TestSchemaColumns_MatchFreshDatabase(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(ctx, filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()
	db := s.(*storeImpl).dbWrite

	if len(schemaColumns) == 0 {
		t.Fatal("no tables parsed from schema.sql")
	}
	for table, defs := range schemaColumns {
		actual, err := pragmaColumns(db, table)
		if err != nil {
			t.Errorf("pragma_table_info(%s): %v", table, err)
			continue
		}
		if len(defs) != len(actual) {
			t.Errorf("%s: parsed %d columns %v, SQLite created %d %v",
				table, len(defs), columnNames(defs), len(actual), actual)
		}
		actualSet := map[string]bool{}
		for _, c := range actual {
			actualSet[c] = true
		}
		for _, def := range defs {
			if !actualSet[def.name] {
				t.Errorf("%s.%s parsed from schema.sql but absent from the created table",
					table, def.name)
			}
		}
	}

	for _, table := range []string{"symbols", "file_index", "calls", "flows", "violations",
		"dependencies", "test_results", "symbol_usage", "directory_hashes"} {
		if !schemaTables[table] {
			t.Errorf("table %q not parsed from schema.sql", table)
		}
	}
	for _, virtual := range []string{"symbols_fts", "vec_symbols"} {
		if schemaTables[virtual] {
			t.Errorf("virtual table %q must not be on the convergence ladder", virtual)
		}
	}
}

func columnNames(defs []columnDef) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.name
	}
	return names
}

// Pins the parsing rules the ladder depends on, on a schema the test owns.
func TestParseSchemaColumns_Rules(t *testing.T) {
	schema := `
CREATE TABLE sample (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alpha TEXT DEFAULT 'a,b' NOT NULL,
    beta REAL DEFAULT 0.5,
    UNIQUE(alpha, beta),
    CONSTRAINT fk_alpha FOREIGN KEY(alpha) REFERENCES other(alpha),
    CHECK (beta IN (1, 2, 3)),
    "quoted_col" TEXT,
    [bracketed] INTEGER
);

CREATE VIRTUAL TABLE sample_fts USING fts5(alpha, beta);

CREATE TABLE second (only TEXT);
`
	got := parseSchemaColumns(schema)

	if len(got) != 2 {
		t.Fatalf("expected 2 real tables, got %d: %v", len(got), got)
	}
	if _, ok := got["sample_fts"]; ok {
		t.Error("virtual table must be skipped")
	}

	want := []string{"id", "alpha", "beta", "quoted_col", "bracketed"}
	if names := columnNames(got["sample"]); len(names) != len(want) {
		t.Fatalf("sample columns = %v, want %v", names, want)
	} else {
		for i, name := range want {
			if names[i] != name {
				t.Errorf("sample column %d = %q, want %q", i, names[i], name)
			}
		}
	}

	// A comma inside a quoted default or inside a constraint must not split.
	for _, def := range got["sample"] {
		if def.name == "alpha" && !strings.Contains(def.ddl, "'a,b'") {
			t.Errorf("alpha ddl lost its quoted default: %q", def.ddl)
		}
	}
	if len(got["second"]) != 1 || got["second"][0].name != "only" {
		t.Errorf("second = %v, want exactly [only]", columnNames(got["second"]))
	}
}

// Pins the cost convergence adds to every store open. `scouter` is a CLI that
// opens the store once per invocation, so this is a per-command tax and a
// regression here is invisible in the functional tests. Reading the columns one
// query at a time cost ~2.7 ms per open; reading each table once keeps it well
// under a millisecond.
func BenchmarkNewStoreOpen(b *testing.B) {
	ctx := context.Background()
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	first, err := NewStore(ctx, dbPath)
	if err != nil {
		b.Fatalf("initial NewStore: %v", err)
	}
	first.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := NewStore(ctx, dbPath)
		if err != nil {
			b.Fatalf("NewStore: %v", err)
		}
		s.Close()
	}
}

func BenchmarkNewStoreOpenOLDDatabase(b *testing.B) {
	ctx := context.Background()
	dbPath := filepath.Join(b.TempDir(), "bench-old.db")
	writeOldDatabase(b, dbPath)

	first, err := NewStore(ctx, dbPath)
	if err != nil {
		b.Fatalf("initial NewStore: %v", err)
	}
	first.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := NewStore(ctx, dbPath)
		if err != nil {
			b.Fatalf("NewStore: %v", err)
		}
		s.Close()
	}
}
