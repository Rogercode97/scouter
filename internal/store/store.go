package store

import (
	"database/sql"
	"os"
	"path/filepath"

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

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
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
		if _, err := db.Exec(q); err != nil {
			return nil, err
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) GetFileIndex(path string) (*FileIndex, error) {
	var idx FileIndex
	query := "SELECT path, mtime, hash, ast_json, project FROM file_index WHERE path = ?"
	err := s.db.QueryRow(query, path).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.ASTJSON, &idx.Project)
	if err != nil {
		return nil, err
	}
	return &idx, nil
}

func (s *Store) SaveFileIndex(idx *FileIndex) error {
	query := `
	INSERT OR REPLACE INTO file_index (path, mtime, hash, ast_json, project)
	VALUES (?, ?, ?, ?, ?);
	`
	_, err := s.db.Exec(query, idx.Path, idx.Mtime, idx.Hash, idx.ASTJSON, idx.Project)
	return err
}

func (s *Store) ClearSymbols(path string) error {
	_, err := s.db.Exec("DELETE FROM symbols WHERE path = ?", path)
	return err
}

func (s *Store) SaveSymbol(sym *Symbol) error {
	query := `
	INSERT INTO symbols (name, type, path, start_byte, end_byte, start_line, end_line)
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.db.Exec(query, sym.Name, sym.Type, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.EndLine)
	return err
}

func (s *Store) SearchSymbols(query string, symType string) ([]Symbol, error) {
	var results []Symbol
	var sqlQuery string
	var args []interface{}

	if symType != "" {
		sqlQuery = `
		SELECT s.name, s.type, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
		FROM symbols s
		JOIN symbols_fts f ON s.id = f.rowid
		WHERE symbols_fts MATCH ? AND s.type = ?
		ORDER BY rank;
		`
		args = append(args, query, symType)
	} else {
		sqlQuery = `
		SELECT s.name, s.type, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
		FROM symbols s
		JOIN symbols_fts f ON s.id = f.rowid
		WHERE symbols_fts MATCH ?
		ORDER BY rank;
		`
		args = append(args, query)
	}

	rows, err := s.db.Query(sqlQuery, args...)
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
