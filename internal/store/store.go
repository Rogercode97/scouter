package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

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
	Name      string `json:"name"`
	Type      string `json:"type"` // Generic Universal: function, class, variable, method
	Path      string `json:"path"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// Repository defines the port for symbol persistence (Hexagonal Architecture)
type Repository interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string) ([]Symbol, error)
	GetStats(ctx context.Context) (int, int, error)
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
			path,
			content='symbols',
			content_rowid='id'
		);`,
		// Triggers to keep FTS5 in sync
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
			INSERT INTO symbols_fts(rowid, name, type, path) VALUES (new.id, new.name, new.type, new.path);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, type, path) VALUES('delete', old.id, old.name, old.type, old.path);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, type, path) VALUES('delete', old.id, old.name, old.type, old.path);
			INSERT INTO symbols_fts(rowid, name, type, path) VALUES (new.id, new.name, new.type, new.path);
		END;`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
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
	INSERT INTO symbols (name, type, path, start_byte, end_byte, start_line, end_line)
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.exec(ctx, query, sym.Name, sym.Type, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.EndLine)
	return err
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
		SELECT s.name, s.type, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
		FROM symbols s
		JOIN symbols_fts f ON s.id = f.rowid
		WHERE symbols_fts MATCH ? AND s.type = ?
		ORDER BY rank
		LIMIT 100;
		`
		args = append(args, safeQuery, symType)
	} else {
		sqlQuery = `
		SELECT s.name, s.type, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
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
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, err
		}
		results = append(results, sym)
	}

	return results, nil
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
