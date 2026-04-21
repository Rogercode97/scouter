package store

import (
	"context"
	"database/sql"
	"fmt"
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

type CriticalSymbol struct {
	Symbol
	Centrality int `json:"centrality"` // Number of incoming calls
	Fragility  int `json:"fragility"`  // Number of failed tests
}

type Call struct {
	CallerName string `json:"caller_name"`
	CalleeName string `json:"callee_name"`
	CalleePath string `json:"callee_path"` // Added for Impact Analysis
	LinkType   string `json:"link_type"`   // Added for Impact Analysis
	Path       string `json:"path"`
	Line       int    `json:"line"`
}

// Repository defines the interface for the Scouter database operations.
// It supports indexing files, symbols, calls, and dependencies.
type Repository interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string) ([]Symbol, error)
	SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error]
	GetSymbolsByRange(ctx context.Context, path string, startLine, endLine int) ([]Symbol, error)
	GetCriticalSymbols(ctx context.Context, limit int) ([]CriticalSymbol, error)
	ResolveInterfaces(ctx context.Context) error
	SaveCall(ctx context.Context, call Call) error
	GetCallers(ctx context.Context, calleeName string) ([]Call, error)
	GetCallees(ctx context.Context, callerName string) ([]Call, error)
	GetImpact(ctx context.Context, symbolName string, filePath string, maxDepth int) ([]types.ImpactResult, error)
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
	WithTransaction(ctx context.Context, fn func(context.Context, Repository) error) error
	Close() error
}

type Store struct {
	db *sql.DB
	tx *sql.Tx
}

var _ Repository = (*Store)(nil)

func New(ctx context.Context, dbPath string) (Repository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Task 1: Move pragmas to DSN
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Task 2: defer Rollback for safety
	defer tx.Rollback()

	if err := migrate(ctx, tx); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit migration: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_index (path TEXT PRIMARY KEY, mtime INTEGER, hash TEXT, ast_json TEXT, project TEXT);`,
		`CREATE TABLE IF NOT EXISTS symbols (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, doc TEXT, path TEXT, start_byte INTEGER, end_byte INTEGER, start_line INTEGER, end_line INTEGER, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, type, doc, path, content='symbols', content_rowid='id');`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
		`CREATE TABLE IF NOT EXISTS calls (id INTEGER PRIMARY KEY AUTOINCREMENT, caller_name TEXT NOT NULL, callee_name TEXT NOT NULL, path TEXT NOT NULL, line INTEGER NOT NULL, callee_path TEXT DEFAULT '', link_type TEXT DEFAULT 'call', FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_impact ON calls(callee_name, callee_path);`,
		`CREATE TABLE IF NOT EXISTS dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version TEXT, type TEXT, project TEXT, direct INTEGER);`,
		`CREATE TABLE IF NOT EXISTS test_results (id INTEGER PRIMARY KEY AUTOINCREMENT, test_name TEXT NOT NULL, status TEXT NOT NULL, error_message TEXT, stack_trace TEXT, target_symbol TEXT, duration_ms INTEGER, project TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE INDEX IF NOT EXISTS idx_test_results_symbol ON test_results(target_symbol);`,
	}

	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	// Universal Migration: Ensure columns exist in base table
	hasDoc, err := hasColumn(ctx, tx, "symbols", "doc")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.doc: %w", err)
	}
	if !hasDoc {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN doc TEXT;"); err != nil {
			return fmt.Errorf("failed to alter table symbols: %w", err)
		}
		// Re-create triggers because they depend on the 'doc' column
		triggerQueries := []string{
			`DROP TRIGGER IF EXISTS symbols_ai;`,
			`DROP TRIGGER IF EXISTS symbols_ad;`,
			`DROP TRIGGER IF EXISTS symbols_au;`,
			`CREATE TRIGGER symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
			`CREATE TRIGGER symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); END;`,
			`CREATE TRIGGER symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path); END;`,
		}
		for _, tq := range triggerQueries {
			if _, err := tx.ExecContext(ctx, tq); err != nil {
				return fmt.Errorf("failed to recreate trigger: %w", err)
			}
		}
	}

	return nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name = ?", table)
	var dummy int
	err := tx.QueryRowContext(ctx, query, column).Scan(&dummy)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("querying pragma_table_info for %s: %w", table, err)
}

func (s *Store) exec(ctx context.Context, q string, a ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, q, a...)
	}
	return s.db.ExecContext(ctx, q, a...)
}

func (s *Store) queryRow(ctx context.Context, q string, a ...any) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, q, a...)
	}
	return s.db.QueryRowContext(ctx, q, a...)
}

func (s *Store) query(ctx context.Context, q string, a ...any) (*sql.Rows, error) {
	if s.tx != nil {
		return s.tx.QueryContext(ctx, q, a...)
	}
	return s.db.QueryContext(ctx, q, a...)
}

func (s *Store) GetFileIndex(ctx context.Context, p string) (*FileIndex, error) {
	var idx FileIndex
	err := s.queryRow(ctx, "SELECT path, mtime, hash, ast_json, project FROM file_index WHERE path = ?", p).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.ASTJSON, &idx.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get file index: %w", err)
	}
	return &idx, nil
}

func (s *Store) SaveFileIndex(ctx context.Context, idx *FileIndex) error {
	query := `INSERT INTO file_index (path, mtime, hash, ast_json, project) 
              VALUES (?, ?, ?, ?, ?) 
              ON CONFLICT(path) DO UPDATE SET 
                mtime=excluded.mtime, 
                hash=excluded.hash, 
                ast_json=excluded.ast_json, 
                project=excluded.project`
	_, err := s.exec(ctx, query, idx.Path, idx.Mtime, idx.Hash, idx.ASTJSON, idx.Project)
	if err != nil {
		return fmt.Errorf("failed to save file index: %w", err)
	}
	return nil
}

func (s *Store) ClearSymbols(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM symbols WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to clear symbols: %w", err)
	}
	return nil
}

func (s *Store) SaveSymbol(ctx context.Context, sym *Symbol) error {
	_, err := s.exec(ctx, "INSERT INTO symbols (name, type, doc, path, start_byte, end_byte, start_line, end_line) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.EndLine)
	if err != nil {
		return fmt.Errorf("failed to save symbol: %w", err)
	}
	return nil
}

func (s *Store) SearchSymbols(ctx context.Context, q, t string) ([]Symbol, error) {
	safe := sanitizeFTS(q)
	if safe == "" {
		return nil, nil
	}
	sql := `SELECT symbols.name, symbols.type, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.end_line 
            FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid 
            WHERE symbols_fts MATCH ?`
	args := []any{safe}
	if t != "" {
		sql += " AND symbols.type = ?"
		args = append(args, t)
	}
	sql += " LIMIT 100"
	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search symbols failed: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) SearchSymbolsWeighted(ctx context.Context, q, t string) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		safe := sanitizeFTS(q)
		if safe == "" {
			return
		}
		sql := `SELECT symbols.name, symbols.type, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.end_line, bm25(symbols_fts, 10.0, 2.0, 1.0, 0.5) as relevance 
                FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid 
                WHERE symbols_fts MATCH ?`
		args := []any{safe}
		if t != "" {
			sql += " AND symbols.type = ?"
			args = append(args, t)
		}
		sql += " ORDER BY relevance ASC LIMIT 100"
		rows, err := s.query(ctx, sql, args...)
		if err != nil {
			yield(Symbol{}, fmt.Errorf("weighted search failed: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var sym Symbol
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine, &sym.Relevance); err != nil {
				if !yield(Symbol{}, fmt.Errorf("scan weighted symbol failed: %w", err)) {
					return
				}
				continue
			}
			if !yield(sym, nil) {
				return
			}
		}
	}
}

func (s *Store) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]Symbol, error) {
	sql := `SELECT name, type, doc, path, start_byte, end_byte, start_line, end_line 
            FROM symbols 
            WHERE path = ? AND NOT (start_line > ? OR end_line < ?)`
	rows, err := s.query(ctx, sql, path, end, start)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols by range: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("scan symbol range failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) GetCriticalSymbols(ctx context.Context, limit int) ([]CriticalSymbol, error) {
	if limit <= 0 {
		limit = 20
	}
	sql := `
		SELECT 
			s.name, s.type, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line,
			COUNT(DISTINCT c.id) as centrality,
			COUNT(DISTINCT tr.id) as fragility
		FROM symbols s
		LEFT JOIN calls c ON c.callee_name = s.name AND (c.callee_path = s.path OR c.callee_path = '')
		LEFT JOIN test_results tr ON tr.target_symbol = s.name AND tr.status = 'fail'
		GROUP BY s.id
		ORDER BY centrality DESC, fragility DESC
		LIMIT ?;`
	
	rows, err := s.query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get critical symbols: %w", err)
	}
	defer rows.Close()

	var results []CriticalSymbol
	for rows.Next() {
		var cs CriticalSymbol
		if err := rows.Scan(&cs.Name, &cs.Type, &cs.Doc, &cs.Path, &cs.StartByte, &cs.EndByte, &cs.StartLine, &cs.EndLine, &cs.Centrality, &cs.Fragility); err != nil {
			return nil, err
		}
		results = append(results, cs)
	}
	return results, nil
}

func (s *Store) ResolveInterfaces(ctx context.Context) error {
	// 1. Get all interfaces and their required methods (method_spec)
	// format: interfaceName -> []methodName
	interfaces := make(map[string][]string)
	rows, err := s.query(ctx, "SELECT name FROM symbols WHERE type = 'method_spec'")
	if err != nil {
		return err
	}
	for rows.Next() {
		var fullName string
		rows.Scan(&fullName)
		parts := strings.Split(fullName, ":")
		if len(parts) == 2 {
			interfaces[parts[0]] = append(interfaces[parts[0]], parts[1])
		}
	}
	rows.Close()

	// 2. Get all structs (class) and their methods
	// format: structName -> []methodName
	structs := make(map[string][]string)
	structPaths := make(map[string]string)
	rows, err = s.query(ctx, "SELECT name, path FROM symbols WHERE type = 'method'")
	if err != nil {
		return err
	}
	for rows.Next() {
		var fullName, path string
		rows.Scan(&fullName, &path)
		parts := strings.Split(fullName, ".")
		if len(parts) == 2 {
			structs[parts[0]] = append(structs[parts[0]], parts[1])
			structPaths[parts[0]] = path
		}
	}
	rows.Close()

	// 3. Match contracts (Duck Typing Engine)
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Repository) error {
		for iface, requiredMethods := range interfaces {
			for strct, actualMethods := range structs {
				if strct == iface { continue } // Skip self-reference if any
				
				matches := 0
				for _, req := range requiredMethods {
					for _, act := range actualMethods {
						if req == act {
							matches++
							break
						}
					}
				}

				// If struct implements all methods of the interface
				if matches == len(requiredMethods) && len(requiredMethods) > 0 {
					// Create implements link: Struct -> Interface
					tx.SaveCall(txCtx, Call{
						CallerName: strct,
						CalleeName: iface,
						Path:       structPaths[strct],
						LinkType:   "implements",
					})
				}
			}
		}
		return nil
	})
}

func (s *Store) SaveCall(ctx context.Context, c Call) error {
	if c.LinkType == "" {
		c.LinkType = "call"
	}
	_, err := s.exec(ctx, "INSERT INTO calls (caller_name, callee_name, path, line, callee_path, link_type) VALUES (?, ?, ?, ?, ?, ?)", c.CallerName, c.CalleeName, c.Path, c.Line, c.CalleePath, c.LinkType)
	if err != nil {
		return fmt.Errorf("failed to save call: %w", err)
	}
	return nil
}

func (s *Store) GetCallers(ctx context.Context, callee string) ([]Call, error) {
	rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line, callee_path, link_type FROM calls WHERE callee_name = ? LIMIT 500", callee)
	if err != nil {
		return nil, fmt.Errorf("failed to get callers: %w", err)
	}
	defer rows.Close()
	var res []Call
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line, &c.CalleePath, &c.LinkType); err != nil {
			return nil, fmt.Errorf("scan caller failed: %w", err)
		}
		res = append(res, c)
	}
	return res, nil
}

func (s *Store) GetCallees(ctx context.Context, caller string) ([]Call, error) {
	rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line, callee_path, link_type FROM calls WHERE caller_name = ? LIMIT 500", caller)
	if err != nil {
		return nil, fmt.Errorf("failed to get callees: %w", err)
	}
	defer rows.Close()
	var res []Call
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line, &c.CalleePath, &c.LinkType); err != nil {
			return nil, fmt.Errorf("scan callee failed: %w", err)
		}
		res = append(res, c)
	}
	return res, nil
}

func (s *Store) GetImpact(ctx context.Context, symbol string, path string, maxDepth int) ([]types.ImpactResult, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	query := `
	WITH RECURSIVE blast_radius(caller_name, caller_path, distance, link_type) AS (
		SELECT caller_name, path, 1, link_type
		FROM calls
		WHERE callee_name = ? AND (callee_path = ? OR callee_path = '')
		
		UNION
		
		SELECT c.caller_name, c.path, br.distance + 1, c.link_type
		FROM calls c
		JOIN blast_radius br ON c.callee_name = br.caller_name AND (c.callee_path = br.caller_path OR c.callee_path = '')
		WHERE br.distance < ?
	)
	SELECT DISTINCT caller_name, caller_path, distance, link_type 
	FROM blast_radius 
	ORDER BY distance ASC LIMIT 500;`

	rows, err := s.query(ctx, query, symbol, path, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("impact analysis failed: %w", err)
	}
	defer rows.Close()

	var results []types.ImpactResult
	for rows.Next() {
		var r types.ImpactResult
		if err := rows.Scan(&r.Symbol, &r.File, &r.Distance, &r.LinkType); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Store) ClearCalls(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM calls WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to clear calls: %w", err)
	}
	return nil
}

func (s *Store) SaveDependency(ctx context.Context, d *types.Dependency) error {
	dir := 0
	if d.Direct {
		dir = 1
	}
	_, err := s.exec(ctx, "INSERT INTO dependencies (name, version, type, project, direct) VALUES (?, ?, ?, ?, ?)", d.Name, d.Version, d.Type, d.Project, dir)
	if err != nil {
		return fmt.Errorf("failed to save dependency: %w", err)
	}
	return nil
}

func (s *Store) GetDependencies(ctx context.Context) ([]types.Dependency, error) {
	rows, err := s.query(ctx, "SELECT name, version, type, project, direct FROM dependencies ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}
	defer rows.Close()
	var res []types.Dependency
	for rows.Next() {
		var d types.Dependency
		var dir int
		if err := rows.Scan(&d.Name, &d.Version, &d.Type, &d.Project, &dir); err != nil {
			return nil, fmt.Errorf("scan dependency failed: %w", err)
		}
		d.Direct = dir == 1
		res = append(res, d)
	}
	return res, nil
}

func (s *Store) ClearDependencies(ctx context.Context) error {
	_, err := s.exec(ctx, "DELETE FROM dependencies")
	if err != nil {
		return fmt.Errorf("failed to clear dependencies: %w", err)
	}
	return nil
}

func (s *Store) GetUnusedSymbols(ctx context.Context, exp bool) ([]Symbol, error) {
	sql := `SELECT name, type, doc, path, start_byte, end_byte, start_line, end_line 
            FROM symbols 
            WHERE NOT EXISTS (SELECT 1 FROM calls WHERE callee_name = symbols.name) 
              AND name NOT IN ('main', 'init') 
              AND path NOT LIKE '%_test.go' 
              AND path NOT LIKE '%main.go'`

	if !exp {
		sql += " AND (name GLOB '[a-z]*')"
	}

	sql += " LIMIT 100"
	rows, err := s.query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to get unused symbols: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("scan unused symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func sanitizeFTS(q string) string {
	if q == "" {
		return ""
	}
	pre := strings.HasSuffix(q, "*")
	s := strings.ReplaceAll(strings.TrimSuffix(q, "*"), "\"", "\"\"")
	s = strings.TrimLeft(s, "*")
	res := "\"" + s + "\""
	if pre {
		res += "*"
	}
	return res
}

func (s *Store) SaveTestResult(ctx context.Context, r *types.TestResult) error {
	_, err := s.exec(ctx, "INSERT INTO test_results (test_name, status, error_message, stack_trace, target_symbol, duration_ms, project) VALUES (?, ?, ?, ?, ?, ?, ?)", r.TestName, r.Status, r.ErrorMessage, r.StackTrace, r.TargetSymbol, r.DurationMS, r.Project)
	if err != nil {
		return fmt.Errorf("failed to save test result: %w", err)
	}
	return nil
}

func (s *Store) GetHealthReport(ctx context.Context, sym string, fails bool) iter.Seq2[types.TestResult, error] {
	return func(yield func(types.TestResult, error) bool) {
		sql := "SELECT test_name, status, error_message, stack_trace, target_symbol, duration_ms, project FROM test_results WHERE 1=1"
		var args []any
		if sym != "" {
			sql += " AND target_symbol = ?"
			args = append(args, sym)
		}
		if fails {
			sql += " AND status = 'fail'"
		}
		rows, err := s.query(ctx, sql+" ORDER BY created_at DESC LIMIT 50", args...)
		if err != nil {
			yield(types.TestResult{}, fmt.Errorf("failed to get health report: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var r types.TestResult
			if err := rows.Scan(&r.TestName, &r.Status, &r.ErrorMessage, &r.StackTrace, &r.TargetSymbol, &r.DurationMS, &r.Project); err != nil {
				yield(types.TestResult{}, fmt.Errorf("scan health report failed: %w", err))
				return
			}
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (s *Store) ClearTestResults(ctx context.Context) error {
	_, err := s.exec(ctx, "DELETE FROM test_results")
	if err != nil {
		return fmt.Errorf("failed to clear test results: %w", err)
	}
	return nil
}

func (s *Store) GetStats(ctx context.Context) (int, int, error) {
	var fc, sc int
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM file_index").Scan(&fc); err != nil {
		return 0, 0, fmt.Errorf("failed to get file count: %w", err)
	}
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM symbols").Scan(&sc); err != nil {
		return 0, 0, fmt.Errorf("failed to get symbol count: %w", err)
	}
	return fc, sc, nil
}

func (s *Store) GetAllFilePaths(ctx context.Context) ([]string, error) {
	rows, err := s.query(ctx, "SELECT path FROM file_index")
	if err != nil {
		return nil, fmt.Errorf("failed to get all file paths: %w", err)
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan file path failed: %w", err)
		}
		res = append(res, p)
	}
	return res, nil
}

func (s *Store) DeleteFileIndex(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM file_index WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to delete file index: %w", err)
	}
	return nil
}

func (s *Store) WithTransaction(ctx context.Context, fn func(context.Context, Repository) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(ctx, &Store{db: s.db, tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }
