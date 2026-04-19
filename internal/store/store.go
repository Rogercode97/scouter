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
	Type      string  `json:"type"` // Generic Universal: function, class, variable, method
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

// Repository defines the port for symbol persistence (Hexagonal Architecture)
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

	// GetAllFilePaths retrieves all file paths currently indexed in the database.
	GetAllFilePaths(ctx context.Context) ([]string, error)

	// DeleteFileIndex removes a file and all its associated symbols and calls from the database.
	DeleteFileIndex(ctx context.Context, path string) error

	// Dependency Sovereignty
	SaveDependency(ctx context.Context, dep *types.Dependency) error
	GetDependencies(ctx context.Context) ([]types.Dependency, error)
	ClearDependencies(ctx context.Context) error

	// Unused Code Detection
	GetUnusedSymbols(ctx context.Context, includeExported bool) ([]Symbol, error)

	WithTransaction(ctx context.Context, fn func(Repository) error) error
	Close() error
}

type Store struct {
	db *sql.DB
	tx *sql.Tx
}

// Ensure Store implements Repository
var _ Repository = (*Store)(nil)

func New(ctx context.Context, dbPath string) (Repository, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Concurrency Upgrade: Performance and Integrity
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return nil, err
		}
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_index (
			path TEXT PRIMARY KEY,
			mtime INTEGER,
			hash TEXT,
			ast_json TEXT,
			project TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			type TEXT,
			doc TEXT,
			path TEXT,
			start_byte INTEGER,
			end_byte INTEGER,
			start_line INTEGER,
			end_line INTEGER,
			FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
			name,
			type,
			doc,
			path,
			content='symbols',
			content_rowid='id'
		);`,
		// Triggers to keep FTS5 in sync
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
			INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, type, doc, path) VALUES('delete', old.id, old.name, old.type, old.doc, old.path);
			INSERT INTO symbols_fts(rowid, name, type, doc, path) VALUES (new.id, new.name, new.type, new.doc, new.path);
		END;`,
		`CREATE TABLE IF NOT EXISTS calls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			caller_name TEXT NOT NULL,
			callee_name TEXT NOT NULL,
			path TEXT NOT NULL,
			line INTEGER NOT NULL,
			FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_caller ON calls(caller_name);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_path ON calls(path);`,
		`CREATE TABLE IF NOT EXISTS dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version TEXT,
			type TEXT,
			project TEXT,
			direct INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_deps_name ON dependencies(name);`,
		`CREATE INDEX IF NOT EXISTS idx_deps_type ON dependencies(type);`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			// Special handling for schema evolution (adding 'doc')
			if strings.Contains(q, "CREATE TABLE IF NOT EXISTS symbols") {
				_, _ = db.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN doc TEXT;")
				_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS symbols_fts;")
				// Continue to recreate properly in the next loop or manual fix
			}
			return nil, err
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, query, args...)
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *Store) query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if s.tx != nil {
		return s.tx.QueryContext(ctx, query, args...)
	}
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Store) GetFileIndex(ctx context.Context, path string) (*FileIndex, error) {
	var idx FileIndex
	query := "SELECT path, mtime, hash, ast_json, project FROM file_index WHERE path = ?"
	err := s.queryRow(ctx, query, path).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.ASTJSON, &idx.Project)
	if err != nil {
		return nil, err
	}
	return &idx, nil
}

func (s *Store) SaveFileIndex(ctx context.Context, idx *FileIndex) error {
	query := `
	INSERT OR REPLACE INTO file_index (path, mtime, hash, ast_json, project)
	VALUES (?, ?, ?, ?, ?);
	`
	_, err := s.exec(ctx, query, idx.Path, idx.Mtime, idx.Hash, idx.ASTJSON, idx.Project)
	return err
}

func (s *Store) ClearSymbols(ctx context.Context, path string) error {
	_, err := s.exec(ctx, "DELETE FROM symbols WHERE path = ?", path)
	return err
}

func (s *Store) SaveSymbol(ctx context.Context, sym *Symbol) error {
	query := `
	INSERT INTO symbols (name, type, doc, path, start_byte, end_byte, start_line, end_line)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.exec(ctx, query, sym.Name, sym.Type, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.EndLine)
	return err
}

func (s *Store) SaveCall(ctx context.Context, call Call) error {
	query := `
	INSERT INTO calls (caller_name, callee_name, path, line)
	VALUES (?, ?, ?, ?);
	`
	_, err := s.exec(ctx, query, call.CallerName, call.CalleeName, call.Path, call.Line)
	return err
}

func (s *Store) GetCallers(ctx context.Context, calleeName string) ([]Call, error) {
	var results []Call
	query := `
	SELECT caller_name, callee_name, path, line
	FROM calls
	WHERE callee_name = ?
	ORDER BY id ASC
	LIMIT 500;
	`
	rows, err := s.query(ctx, query, calleeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var call Call
		if err := rows.Scan(&call.CallerName, &call.CalleeName, &call.Path, &call.Line); err != nil {
			return nil, err
		}
		results = append(results, call)
	}

	return results, nil
}

func (s *Store) GetCallees(ctx context.Context, callerName string) ([]Call, error) {
	var results []Call
	query := `
	SELECT caller_name, callee_name, path, line
	FROM calls
	WHERE caller_name = ?
	ORDER BY id ASC
	LIMIT 500;
	`
	rows, err := s.query(ctx, query, callerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var call Call
		if err := rows.Scan(&call.CallerName, &call.CalleeName, &call.Path, &call.Line); err != nil {
			return nil, err
		}
		results = append(results, call)
	}

	return results, nil
}

func (s *Store) ClearCalls(ctx context.Context, path string) error {
	_, err := s.exec(ctx, "DELETE FROM calls WHERE path = ?", path)
	return err
}

func (s *Store) SaveDependency(ctx context.Context, dep *types.Dependency) error {
	query := `
	INSERT INTO dependencies (name, version, type, project, direct)
	VALUES (?, ?, ?, ?, ?);
	`
	directInt := 0
	if dep.Direct {
		directInt = 1
	}
	_, err := s.exec(ctx, query, dep.Name, dep.Version, dep.Type, dep.Project, directInt)
	return err
}

func (s *Store) GetDependencies(ctx context.Context) ([]types.Dependency, error) {
	var results []types.Dependency
	query := "SELECT name, version, type, project, direct FROM dependencies ORDER BY name ASC"
	rows, err := s.query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dep types.Dependency
		var directInt int
		if err := rows.Scan(&dep.Name, &dep.Version, &dep.Type, &dep.Project, &directInt); err != nil {
			return nil, err
		}
		dep.Direct = directInt == 1
		results = append(results, dep)
	}
	return results, nil
}

func (s *Store) ClearDependencies(ctx context.Context) error {
	_, err := s.exec(ctx, "DELETE FROM dependencies")
	return err
}

func (s *Store) GetUnusedSymbols(ctx context.Context, includeExported bool) ([]Symbol, error) {
	var results []Symbol
	query := `
	SELECT name, type, doc, path, start_byte, end_byte, start_line, end_line
	FROM symbols s
	WHERE NOT EXISTS (SELECT 1 FROM calls c WHERE c.callee_name = s.name)
	AND name NOT IN ('main', 'init')
	AND path NOT LIKE '%_test.go'
	AND path NOT LIKE '%main.go'
	`
	if !includeExported {
		// Filter out symbols starting with Uppercase (Go export convention)
		query += " AND name GLOB '[a-z]*'"
	}
	query += " ORDER BY name ASC LIMIT 100;"

	rows, err := s.query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, err
		}
		results = append(results, sym)
	}
	return results, nil
}

func sanitizeFTS(query string) string {
	// Check for prefix search
	isPrefix := strings.HasSuffix(query, "*")
	s := query
	if isPrefix {
		s = strings.TrimSuffix(s, "*")
	}

	// Escape existing double quotes
	s = strings.ReplaceAll(s, "\"", "\"\"")

	// Remove leading * which breaks FTS5 even inside quotes sometimes or is just invalid
	s = strings.TrimLeft(s, "*")

	// Wrap in double quotes
	result := "\"" + s + "\""

	// Append * if it was a prefix search
	if isPrefix {
		result += "*"
	}

	return result
}

func (s *Store) SearchSymbols(ctx context.Context, query string, symType string) ([]Symbol, error) {
	var results []Symbol
	var sqlQuery string
	var args []interface{}

	safeQuery := sanitizeFTS(query)

	if symType != "" {
		sqlQuery = `
		SELECT s.name, s.type, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
		FROM symbols s
		JOIN symbols_fts f ON s.id = f.rowid
		WHERE symbols_fts MATCH ? AND s.type = ?
		ORDER BY rank
		LIMIT 100;
		`
		args = append(args, safeQuery, symType)
	} else {
		sqlQuery = `
		SELECT s.name, s.type, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
		FROM symbols s
		JOIN symbols_fts f ON s.id = f.rowid
		WHERE symbols_fts MATCH ?
		ORDER BY rank
		LIMIT 100;
		`
		args = append(args, safeQuery)
	}

	rows, err := s.query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, err
		}
		results = append(results, sym)
	}

	return results, nil
}

func (s *Store) SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		safeQuery := sanitizeFTS(query)
		var sqlQuery string
		var args []interface{}

		// Weights: name (10.0), type (2.0), doc (1.0), path (0.5)
		if symType != "" {
			sqlQuery = `
			SELECT s.name, s.type, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line, bm25(symbols_fts, 10.0, 2.0, 1.0, 0.5) as relevance
			FROM symbols s
			JOIN symbols_fts f ON s.id = f.rowid
			WHERE symbols_fts MATCH ? AND s.type = ?
			ORDER BY relevance ASC
			LIMIT 100;
			`
			args = append(args, safeQuery, symType)
		} else {
			sqlQuery = `
			SELECT s.name, s.type, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line, bm25(symbols_fts, 10.0, 2.0, 1.0, 0.5) as relevance
			FROM symbols s
			JOIN symbols_fts f ON s.id = f.rowid
			WHERE symbols_fts MATCH ?
			ORDER BY relevance ASC
			LIMIT 100;
			`
			args = append(args, safeQuery)
		}

		rows, err := s.query(ctx, sqlQuery, args...)
		if err != nil {
			yield(Symbol{}, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var sym Symbol
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine, &sym.Relevance); err != nil {
				if !yield(Symbol{}, err) {
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

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetStats(ctx context.Context) (int, int, error) {
	var fileCount, symbolCount int
	err := s.queryRow(ctx, "SELECT COUNT(*) FROM file_index").Scan(&fileCount)
	if err != nil {
		return 0, 0, err
	}
	err = s.queryRow(ctx, "SELECT COUNT(*) FROM symbols").Scan(&symbolCount)
	if err != nil {
		return 0, 0, err
	}
	return fileCount, symbolCount, nil
}

func (s *Store) GetAllFilePaths(ctx context.Context) ([]string, error) {
	var paths []string
	rows, err := s.query(ctx, "SELECT path FROM file_index")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (s *Store) DeleteFileIndex(ctx context.Context, path string) error {
	_, err := s.exec(ctx, "DELETE FROM file_index WHERE path = ?", path)
	return err
}

func (s *Store) WithTransaction(ctx context.Context, fn func(Repository) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create a temporary store wrapped in the transaction
	txStore := &Store{db: s.db, tx: tx}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit()
}
