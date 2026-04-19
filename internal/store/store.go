package store

import (
	"context"
	"database/sql"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	_ "modernc.org/sqlite"
)

type FileIndex struct {
	Path    string `json:"path"`
	Mtime   int64  `json:"mtime"`
	Hash    string `json:"hash"`
	ASTJSON string `json:"ast_json"`
	Project string `json:"project"`
}

type Symbol struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"` 
	Doc       string  `json:"doc"`
	Path      string  `json:"path"`
	StartByte int     `json:"start_byte"`
	EndByte   int     `json:"end_byte"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Relevance float64 `json:"relevance,omitempty"`
}

type Call struct {
	CallerName string `json:"caller_name"`
	CalleeName string `json:"callee_name"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
}

type Repository interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string) ([]Symbol, error)
	SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error]
	SaveCall(ctx context.Context, call Call) error
	GetCallers(ctx context.Context, calleeName string) ([]Call, error)
	GetCallees(ctx context.Context, callerName string) ([]Call, error)
	ClearCalls(ctx context.Context, path string) error
	GetStats(ctx context.Context) (int, int, error)
	GetAllFilePaths(ctx context.Context) ([]string, error)
	DeleteFileIndex(ctx context.Context, path string) error
	SaveDependency(ctx context.Context, dep *types.Dependency) error
	GetDependencies(ctx context.Context) ([]types.Dependency, error)
	ClearDependencies(ctx context.Context) error
	GetUnusedSymbols(ctx context.Context, includeExported bool) ([]Symbol, error)
	SaveTestResult(ctx context.Context, res *types.TestResult) error
	GetHealthReport(ctx context.Context, symbol string, failuresOnly bool) iter.Seq2[types.TestResult, error]
	ClearTestResults(ctx context.Context) error
	WithTransaction(ctx context.Context, fn func(Repository) error) error
	Close() error
}

type Store struct {
	db *sql.DB
	tx *sql.Tx
}

var _ Repository = (*Store)(nil)

func New(ctx context.Context, dbPath string) (Repository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil { return nil, err }
	db, err := sql.Open("sqlite", dbPath)
	if err != nil { return nil, err }

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil { return nil, err }
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_index (path TEXT PRIMARY KEY, mtime INTEGER, hash TEXT, ast_json TEXT, project TEXT);`,
		`CREATE TABLE IF NOT EXISTS symbols (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, doc TEXT, path TEXT, start_byte INTEGER, end_byte INTEGER, start_line INTEGER, end_line INTEGER, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, type, doc, path, content='symbols', content_rowid='id');`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
		`CREATE TABLE IF NOT EXISTS calls (id INTEGER PRIMARY KEY AUTOINCREMENT, caller_name TEXT NOT NULL, callee_name TEXT NOT NULL, path TEXT NOT NULL, line INTEGER NOT NULL, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);`,
		`CREATE TABLE IF NOT EXISTS dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version TEXT, type TEXT, project TEXT, direct INTEGER);`,
		`CREATE TABLE IF NOT EXISTS test_results (id INTEGER PRIMARY KEY AUTOINCREMENT, test_name TEXT NOT NULL, status TEXT NOT NULL, error_message TEXT, stack_trace TEXT, target_symbol TEXT, duration_ms INTEGER, project TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			if strings.Contains(q, "CREATE TABLE IF NOT EXISTS symbols") {
				_, _ = db.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN doc TEXT;")
			}
			if strings.Contains(q, "CREATE VIRTUAL TABLE") {
				_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS symbols_fts;")
				_, _ = db.ExecContext(ctx, q)
			}
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) exec(ctx context.Context, q string, a ...any) (sql.Result, error) {
	if s.tx != nil { return s.tx.ExecContext(ctx, q, a...) }
	return s.db.ExecContext(ctx, q, a...)
}

func (s *Store) queryRow(ctx context.Context, q string, a ...any) *sql.Row {
	if s.tx != nil { return s.tx.QueryRowContext(ctx, q, a...) }
	return s.db.QueryRowContext(ctx, q, a...)
}

func (s *Store) query(ctx context.Context, q string, a ...any) (*sql.Rows, error) {
	if s.tx != nil { return s.tx.QueryContext(ctx, q, a...) }
	return s.db.QueryContext(ctx, q, a...)
}

func (s *Store) GetFileIndex(ctx context.Context, p string) (*FileIndex, error) {
	var idx FileIndex
	err := s.queryRow(ctx, "SELECT path, mtime, hash, ast_json, project FROM file_index WHERE path = ?", p).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.ASTJSON, &idx.Project)
	return &idx, err
}

func (s *Store) SaveFileIndex(ctx context.Context, idx *FileIndex) error {
	_, err := s.exec(ctx, "INSERT OR REPLACE INTO file_index (path, mtime, hash, ast_json, project) VALUES (?, ?, ?, ?, ?)", idx.Path, idx.Mtime, idx.Hash, idx.ASTJSON, idx.Project)
	return err
}

func (s *Store) ClearSymbols(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM symbols WHERE path = ?", p)
	return err
}

func (s *Store) SaveSymbol(ctx context.Context, sym *Symbol) error {
	_, err := s.exec(ctx, "INSERT INTO symbols (name, type, doc, path, start_byte, end_byte, start_line, end_line) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.EndLine)
	return err
}

func (s *Store) SearchSymbols(ctx context.Context, q, t string) ([]Symbol, error) {
	safe := sanitizeFTS(q)
	// UNIVERSAL SYNTAX: Use explicit table names, avoid aliases in JOIN
	sql := `SELECT symbols.name, symbols.type, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.end_line 
            FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid 
            WHERE symbols_fts MATCH ?`
	args := []any{safe}
	if t != "" { sql += " AND symbols.type = ?"; args = append(args, t) }
	sql += " LIMIT 100"
	rows, err := s.query(ctx, sql, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil { return nil, err }
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) SearchSymbolsWeighted(ctx context.Context, q, t string) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		safe := sanitizeFTS(q)
		// UNIVERSAL WEIGHTED SYNTAX: bm25 over virtual table, results from base table
		sql := `SELECT symbols.name, symbols.type, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.end_line, bm25(symbols_fts, 10.0, 2.0, 1.0, 0.5) as relevance 
                FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid 
                WHERE symbols_fts MATCH ?`
		args := []any{safe}
		if t != "" { sql += " AND symbols.type = ?"; args = append(args, t) }
		sql += " ORDER BY relevance ASC LIMIT 100"
		rows, err := s.query(ctx, sql, args...)
		if err != nil { yield(Symbol{}, err); return }
		defer rows.Close()
		for rows.Next() {
			var sym Symbol
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine, &sym.Relevance); err != nil {
				if !yield(Symbol{}, err) { return }
				continue
			}
			if !yield(sym, nil) { return }
		}
	}
}

func (s *Store) SaveCall(ctx context.Context, c Call) error {
	_, err := s.exec(ctx, "INSERT INTO calls (caller_name, callee_name, path, line) VALUES (?, ?, ?, ?)", c.CallerName, c.CalleeName, c.Path, c.Line)
	return err
}

func (s *Store) GetCallers(ctx context.Context, callee string) ([]Call, error) {
	rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line FROM calls WHERE callee_name = ? LIMIT 500", callee)
	if err != nil { return nil, err }
	defer rows.Close()
	var res []Call
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line); err != nil { return nil, err }
		res = append(res, c)
	}
	return res, nil
}

func (s *Store) GetCallees(ctx context.Context, caller string) ([]Call, error) {
	rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line FROM calls WHERE caller_name = ? LIMIT 500", caller)
	if err != nil { return nil, err }
	defer rows.Close()
	var res []Call
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line); err != nil { return nil, err }
		res = append(res, c)
	}
	return res, nil
}

func (s *Store) ClearCalls(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM calls WHERE path = ?", p)
	return err
}

func (s *Store) SaveDependency(ctx context.Context, d *types.Dependency) error {
	dir := 0; if d.Direct { dir = 1 }
	_, err := s.exec(ctx, "INSERT INTO dependencies (name, version, type, project, direct) VALUES (?, ?, ?, ?, ?)", d.Name, d.Version, d.Type, d.Project, dir)
	return err
}

func (s *Store) GetDependencies(ctx context.Context) ([]types.Dependency, error) {
	rows, err := s.query(ctx, "SELECT name, version, type, project, direct FROM dependencies ORDER BY name")
	if err != nil { return nil, err }
	defer rows.Close()
	var res []types.Dependency
	for rows.Next() {
		var d types.Dependency; var dir int
		if err := rows.Scan(&d.Name, &d.Version, &d.Type, &d.Project, &dir); err != nil { return nil, err }
		d.Direct = dir == 1; res = append(res, d)
	}
	return res, nil
}

func (s *Store) ClearDependencies(ctx context.Context) error { _, err := s.exec(ctx, "DELETE FROM dependencies"); return err }

func (s *Store) GetUnusedSymbols(ctx context.Context, exp bool) ([]Symbol, error) {
	sql := `SELECT name, type, doc, path, start_byte, end_byte, start_line, end_line FROM symbols WHERE NOT EXISTS (SELECT 1 FROM calls WHERE callee_name = symbols.name) AND name NOT IN ('main', 'init') AND path NOT LIKE '%_test.go' AND path NOT LIKE '%main.go' LIMIT 100`
	rows, err := s.query(ctx, sql)
	if err != nil { return nil, err }
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil { return nil, err }
		res = append(res, sym)
	}
	return res, nil
}

func sanitizeFTS(q string) string {
	pre := strings.HasSuffix(q, "*"); s := strings.ReplaceAll(strings.TrimSuffix(q, "*"), "\"", "\"\""); s = strings.TrimLeft(s, "*")
	res := "\"" + s + "\""; if pre { res += "*" }; return res
}

func (s *Store) SaveTestResult(ctx context.Context, r *types.TestResult) error {
	_, err := s.exec(ctx, "INSERT INTO test_results (test_name, status, error_message, stack_trace, target_symbol, duration_ms, project) VALUES (?, ?, ?, ?, ?, ?, ?)", r.TestName, r.Status, r.ErrorMessage, r.StackTrace, r.TargetSymbol, r.DurationMS, r.Project)
	return err
}

func (s *Store) GetHealthReport(ctx context.Context, sym string, fails bool) iter.Seq2[types.TestResult, error] {
	return func(yield func(types.TestResult, error) bool) {
		sql := "SELECT test_name, status, error_message, stack_trace, target_symbol, duration_ms, project FROM test_results WHERE 1=1"
		var args []any
		if sym != "" { sql += " AND target_symbol = ?"; args = append(args, sym) }
		if fails { sql += " AND status = 'fail'" }
		rows, err := s.query(ctx, sql + " ORDER BY created_at DESC LIMIT 50", args...)
		if err != nil { yield(types.TestResult{}, err); return }
		defer rows.Close()
		for rows.Next() {
			var r types.TestResult
			if err := rows.Scan(&r.TestName, &r.Status, &r.ErrorMessage, &r.StackTrace, &r.TargetSymbol, &r.DurationMS, &r.Project); err != nil { yield(types.TestResult{}, err); return }
			if !yield(r, nil) { return }
		}
	}
}

func (s *Store) ClearTestResults(ctx context.Context) error { _, err := s.exec(ctx, "DELETE FROM test_results"); return err }

func (s *Store) GetStats(ctx context.Context) (int, int, error) {
	var fc, sc int
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM file_index").Scan(&fc); err != nil { return 0, 0, err }
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM symbols").Scan(&sc); err != nil { return 0, 0, err }
	return fc, sc, nil
}

func (s *Store) GetAllFilePaths(ctx context.Context) ([]string, error) {
	rows, err := s.query(ctx, "SELECT path FROM file_index"); if err != nil { return nil, err }; defer rows.Close()
	var res []string
	for rows.Next() { var p string; if err := rows.Scan(&p); err != nil { return nil, err }; res = append(res, p) }
	return res, nil
}

func (s *Store) DeleteFileIndex(ctx context.Context, p string) error { _, err := s.exec(ctx, "DELETE FROM file_index WHERE path = ?", p); return err }

func (s *Store) WithTransaction(ctx context.Context, fn func(Repository) error) error {
	tx, err := s.db.BeginTx(ctx, nil); if err != nil { return err }; defer tx.Rollback()
	if err := fn(&Store{db: s.db, tx: tx}); err != nil { return err }; return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }
