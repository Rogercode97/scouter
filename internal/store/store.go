package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	_ "modernc.org/sqlite"
)

type FileIndex struct {
	Path      string `json:"path"`
	Mtime     int64  `json:"mtime"`
	Hash      string `json:"hash"`
	ASTJSON   string `json:"ast_json"`
	Project   string `json:"project"`
	Freshness int    `json:"freshness"` // 0: fresh, 1: edited, 2: stale
}

type Symbol struct {
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	PackagePath    string  `json:"package_path"`  // Fully qualified package path
	ReceiverType   string  `json:"receiver_type"` // pointer, value, or empty
	Signature      string  `json:"signature,omitempty"`
	Doc            string  `json:"doc"`
	Path           string  `json:"path"`
	StartByte      int     `json:"start_byte"`
	EndByte        int     `json:"end_byte"`
	StartLine      int     `json:"start_line"`
	StartCol       int     `json:"start_col"`
	EndLine        int     `json:"end_line"`
	StructuralHash string  `json:"structural_hash,omitempty"`
	Relevance      float64 `json:"relevance,omitempty"`
	PageRank       float64 `json:"pagerank,omitempty"`
	ChurnScore     float64 `json:"churn_score,omitempty"`
	AISummary      string  `json:"ai_summary,omitempty"`
}

type CriticalSymbol struct {
	Symbol
	Centrality int `json:"centrality"`
	Fragility  int `json:"fragility"`
}

type Call struct {
	CallerName string `json:"caller_name"`
	CalleeName string `json:"callee_name"`
	CalleePath string `json:"callee_path"`
	LinkType   string `json:"link_type"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
}

// SovereignDelta represents the index data for a single file, stable for Git.
type SovereignDelta struct {
	Path    string   `json:"path"`
	Hash    string   `json:"hash"`
	Symbols []Symbol `json:"symbols"`
	Calls   []Call   `json:"calls"`
}

type BatchItem struct {
	Index      *FileIndex
	Symbols    []Symbol
	Calls      []Call
	Violations []Violation
}

type Violation struct {
	ID        int    `json:"id"`
	RuleID    string `json:"rule_id"`
	FilePath  string `json:"file_path"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	Text      string `json:"text"`
}

// Repository defines the interface for the Scouter database operations.
type Repository interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	SaveFileIndexBatch(ctx context.Context, items []BatchItem) error
	GetDirectoryHash(ctx context.Context, path string) (string, int64, error)
	SaveDirectoryHash(ctx context.Context, path string, hash string, mtime int64) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string, limit, offset int) ([]Symbol, error)
	GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]Symbol, error)
	GetSymbolsByStructuralHash(ctx context.Context, hash string) ([]Symbol, error)
	SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error]
	GetSymbolsByRange(ctx context.Context, path string, startLine, endLine int) ([]Symbol, error)
	GetSymbolsByType(ctx context.Context, symType string) ([]Symbol, error)
	GetInterfaces(ctx context.Context) ([]Symbol, error)
	GetAllSymbols(ctx context.Context) iter.Seq2[Symbol, error]
	GetAllCalls(ctx context.Context) iter.Seq2[Call, error]
	GetAllFailedTests(ctx context.Context) iter.Seq2[types.TestResult, error]
	UpdateSymbolCentrality(ctx context.Context, name, path string, centrality int) error
	UpdateSymbolChurn(ctx context.Context, path string, score float64) error
	UpdateSymbolPageRank(ctx context.Context, name, path string, score float64) error
	ExportDelta(ctx context.Context, syncDir string) error
	ImportDelta(ctx context.Context, syncDir string) error
	SaveCall(ctx context.Context, call Call) error
	GetCallers(ctx context.Context, calleeName string, limit, offset int) ([]Call, error)
	GetCallees(ctx context.Context, callerName string) ([]Call, error)
	GetCallersRecursive(ctx context.Context, name, path string, maxDepth int) ([]Call, error)
	GetAffectedTestsRecursive(ctx context.Context, name, path string) ([]Symbol, error)
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
	SaveViolation(ctx context.Context, v *types.ASTRuleMatch) error
	GetViolationsByFile(ctx context.Context, path string) ([]Violation, error)
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

	// Optimized SQLite parameters for high-performance bulk indexing
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)", dbPath, separator)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := migrate(ctx, tx); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit migration: %w", err)
	}

	s := &Store{
		db: db,
	}

	// Go 1.25 native cleanup
	runtime.AddCleanup(s, func(db *sql.DB) {
		_ = db.Close()
	}, db)

	return s, nil
}

func migrate(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_index (path TEXT PRIMARY KEY, mtime INTEGER, hash TEXT, ast_json TEXT, project TEXT);`,
		`CREATE TABLE IF NOT EXISTS directory_hashes (path TEXT PRIMARY KEY, hash TEXT, mtime INTEGER);`,
		`CREATE TABLE IF NOT EXISTS symbols (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, package_path TEXT DEFAULT '', receiver_type TEXT DEFAULT '', signature TEXT DEFAULT '', doc TEXT, path TEXT, start_byte INTEGER, end_byte INTEGER, start_line INTEGER, start_col INTEGER, end_line INTEGER, structural_hash TEXT DEFAULT '', indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, type, signature, doc, path, content='symbols', content_rowid='id');`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_path ON symbols(path);`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_resolution ON symbols(name, path);`,
		`CREATE TABLE IF NOT EXISTS calls (id INTEGER PRIMARY KEY AUTOINCREMENT, caller_name TEXT NOT NULL, callee_name TEXT NOT NULL, path TEXT NOT NULL, line INTEGER NOT NULL, callee_path TEXT DEFAULT '', link_type TEXT DEFAULT 'call', indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_impact ON calls(callee_name, callee_path);`,
		`CREATE TABLE IF NOT EXISTS dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version TEXT, type TEXT, project TEXT, direct INTEGER);`,
		`CREATE TABLE IF NOT EXISTS test_results (id INTEGER PRIMARY KEY AUTOINCREMENT, test_name TEXT NOT NULL, status TEXT NOT NULL, error_message TEXT, stack_trace TEXT, target_symbol TEXT, duration_ms INTEGER, project TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE INDEX IF NOT EXISTS idx_test_results_symbol ON test_results(target_symbol);`,
		`CREATE TABLE IF NOT EXISTS violations (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_id TEXT, file_path TEXT, message TEXT, severity TEXT, start_line INTEGER, start_col INTEGER, text TEXT, FOREIGN KEY(file_path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE INDEX IF NOT EXISTS idx_violations_file ON violations(file_path);`,
	}

	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	// [Divine Fix] Wave 11 Data Integrity: Remove duplicates before creating unique index
	cleanupQuery := `
		DELETE FROM calls 
		WHERE id NOT IN (
			SELECT MIN(id) 
			FROM calls 
			GROUP BY caller_name, callee_name, path, line, link_type
		);`
	if _, err := tx.ExecContext(ctx, cleanupQuery); err != nil {
		return fmt.Errorf("failed to cleanup duplicate calls: %w", err)
	}

	// [Divine Fix] Wave 11 Data Integrity: Apply unique index after cleanup
	if _, err := tx.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_unique ON calls(caller_name, callee_name, path, line, link_type);"); err != nil {
		return fmt.Errorf("failed to create unique index on calls: %w", err)
	}

	// Dynamic column check for 'doc'
	hasDoc, err := hasColumn(ctx, tx, "symbols", "doc")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.doc: %w", err)
	}
	if !hasDoc {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN doc TEXT;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (doc): %w", err)
		}
	}

	// Dynamic column check for 'signature'
	hasSig, err := hasColumn(ctx, tx, "symbols", "signature")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.signature: %w", err)
	}
	if !hasSig {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN signature TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (signature): %w", err)
		}
	}

	// Dynamic column check for 'signature' in FTS (SQLite FTS5 does not support ALTER TABLE)
	hasFTSig, err := hasColumn(ctx, tx, "symbols_fts", "signature")
	if err != nil {
		return fmt.Errorf("failed to check column symbols_fts.signature: %w", err)
	}
	if !hasFTSig {
		// Drop and recreate FTS table
		ftsQueries := []string{
			`DROP TABLE IF EXISTS symbols_fts;`,
			`CREATE VIRTUAL TABLE symbols_fts USING fts5(name, type, signature, doc, path, content='symbols', content_rowid='id');`,
			`INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) SELECT id, name, type, signature, doc, path FROM symbols;`,
		}
		for _, q := range ftsQueries {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("failed to recreate symbols_fts: %w", err)
			}
		}
	}

	// Recreate triggers to include signature and doc
	triggerQueries := []string{
		`DROP TRIGGER IF EXISTS symbols_ai;`,
		`DROP TRIGGER IF EXISTS symbols_ad;`,
		`DROP TRIGGER IF EXISTS symbols_au;`,
		`CREATE TRIGGER symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE TRIGGER symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); END;`,
		`CREATE TRIGGER symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
	}
	for _, tq := range triggerQueries {
		if _, err := tx.ExecContext(ctx, tq); err != nil {
			return fmt.Errorf("failed to recreate trigger: %w", err)
		}
	}

	// Dynamic column check for 'start_col'
	hasCol, err := hasColumn(ctx, tx, "symbols", "start_col")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.start_col: %w", err)
	}
	if !hasCol {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN start_col INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (start_col): %w", err)
		}
	}

	// Dynamic column check for 'indegree' in 'calls'
	hasIndegree, err := hasColumn(ctx, tx, "calls", "indegree")
	if err != nil {
		return fmt.Errorf("failed to check column calls.indegree: %w", err)
	}
	if !hasIndegree {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE calls ADD COLUMN indegree INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table calls (indegree): %w", err)
		}
	}

	// Dynamic column check for 'indegree' in 'symbols'
	hasSymIndegree, err := hasColumn(ctx, tx, "symbols", "indegree")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.indegree: %w", err)
	}
	if !hasSymIndegree {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN indegree INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (indegree): %w", err)
		}
	}

	// Dynamic column check for 'structural_hash'
	hasStructHash, err := hasColumn(ctx, tx, "symbols", "structural_hash")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.structural_hash: %w", err)
	}
	if !hasStructHash {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN structural_hash TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (structural_hash): %w", err)
		}
		if _, err := tx.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_symbols_structural_hash ON symbols(structural_hash);"); err != nil {
			return fmt.Errorf("failed to create index on structural_hash: %w", err)
		}
	}

	// Dynamic column check for 'package_path'
	hasPkg, err := hasColumn(ctx, tx, "symbols", "package_path")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.package_path: %w", err)
	}
	if !hasPkg {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN package_path TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (package_path): %w", err)
		}
	}

	// Dynamic column check for 'receiver_type'
	hasRec, err := hasColumn(ctx, tx, "symbols", "receiver_type")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.receiver_type: %w", err)
	}
	if !hasRec {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN receiver_type TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (receiver_type): %w", err)
		}
	}

	// Dynamic column check for 'pagerank'
	hasPagerank, err := hasColumn(ctx, tx, "symbols", "pagerank")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.pagerank: %w", err)
	}
	if !hasPagerank {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN pagerank REAL DEFAULT 0.0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (pagerank): %w", err)
		}
	}

	// Dynamic column check for 'churn_score'
	hasChurn, err := hasColumn(ctx, tx, "symbols", "churn_score")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.churn_score: %w", err)
	}
	if !hasChurn {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN churn_score REAL DEFAULT 0.0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (churn_score): %w", err)
		}
	}

	// Dynamic column check for 'ai_summary'
	hasSummary, err := hasColumn(ctx, tx, "symbols", "ai_summary")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.ai_summary: %w", err)
	}
	if !hasSummary {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN ai_summary TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (ai_summary): %w", err)
		}
	}

	// Dynamic column check for 'freshness' in 'file_index'
	hasFresh, err := hasColumn(ctx, tx, "file_index", "freshness")
	if err != nil {
		return fmt.Errorf("failed to check column file_index.freshness: %w", err)
	}
	if !hasFresh {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE file_index ADD COLUMN freshness INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table file_index (freshness): %w", err)
		}
	}

	return nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name = ?", table) // #nosec G201 - table name is internal
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
	err := s.queryRow(ctx, "SELECT path, mtime, hash, ast_json, project, freshness FROM file_index WHERE path = ?", p).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.ASTJSON, &idx.Project, &idx.Freshness)
	if err != nil {
		return nil, fmt.Errorf("failed to get file index: %w", err)
	}
	return &idx, nil
}

func (s *Store) SaveFileIndex(ctx context.Context, idx *FileIndex) error {
	query := `INSERT INTO file_index (path, mtime, hash, ast_json, project, freshness)
              VALUES (?, ?, ?, ?, ?, ?)
              ON CONFLICT(path) DO UPDATE SET
                mtime=excluded.mtime,
                hash=excluded.hash,
                ast_json=excluded.ast_json,
                project=excluded.project,
                freshness=excluded.freshness`
	_, err := s.exec(ctx, query, idx.Path, idx.Mtime, idx.Hash, idx.ASTJSON, idx.Project, idx.Freshness)
	if err != nil {
		return fmt.Errorf("failed to save file index: %w", err)
	}
	return nil
}

func (s *Store) SaveFileIndexBatch(ctx context.Context, items []BatchItem) error {
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Repository) error {
		for _, item := range items {
			if item.Index != nil {
				if err := tx.SaveFileIndex(txCtx, item.Index); err != nil {
					return fmt.Errorf("failed to save index for %s: %w", item.Index.Path, err)
				}
				if err := tx.ClearSymbols(txCtx, item.Index.Path); err != nil {
					return fmt.Errorf("failed to clear symbols for %s: %w", item.Index.Path, err)
				}
				if err := tx.ClearCalls(txCtx, item.Index.Path); err != nil {
					return fmt.Errorf("failed to clear calls for %s: %w", item.Index.Path, err)
				}
			}
			for _, sym := range item.Symbols {
				symCopy := sym
				if err := tx.SaveSymbol(txCtx, &symCopy); err != nil {
					return fmt.Errorf("failed to save symbol %s: %w", symCopy.Name, err)
				}
			}
			for _, call := range item.Calls {
				if err := tx.SaveCall(txCtx, call); err != nil {
					return fmt.Errorf("failed to save call from %s to %s: %w", call.CallerName, call.CalleeName, err)
				}
			}
			for _, violation := range item.Violations {
				vCopy := violation // Create copy to take pointer
				astMatch := &types.ASTRuleMatch{
					RuleID:   vCopy.RuleID,
					File:     vCopy.FilePath,
					Message:  vCopy.Message,
					Severity: vCopy.Severity,
					Range: types.ASTRange{
						Start: types.ASTPos{
							Line:   vCopy.StartLine,
							Column: vCopy.StartCol,
						},
					},
					Text:     vCopy.Text,
				}
				if err := tx.SaveViolation(txCtx, astMatch); err != nil {
					return fmt.Errorf("failed to save violation for %s: %w", vCopy.FilePath, err)
				}
			}
		}
		return nil
	})
}

func (s *Store) GetDirectoryHash(ctx context.Context, p string) (string, int64, error) {
	var hash string
	var mtime int64
	err := s.queryRow(ctx, "SELECT hash, mtime FROM directory_hashes WHERE path = ?", p).Scan(&hash, &mtime)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("failed to get directory hash: %w", err)
	}
	return hash, mtime, nil
}

func (s *Store) SaveDirectoryHash(ctx context.Context, p string, hash string, mtime int64) error {
	query := `INSERT INTO directory_hashes (path, hash, mtime)
              VALUES (?, ?, ?)
              ON CONFLICT(path) DO UPDATE SET
                hash=excluded.hash,
                mtime=excluded.mtime`
	_, err := s.exec(ctx, query, p, hash, mtime)
	if err != nil {
		return fmt.Errorf("failed to save directory hash: %w", err)
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
	_, err := s.exec(ctx, "INSERT INTO symbols (name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, pagerank, churn_score, ai_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.PageRank, sym.ChurnScore, sym.AISummary)
	if err != nil {
		return fmt.Errorf("failed to save symbol: %w", err)
	}
	return nil
}

func (s *Store) SearchSymbols(ctx context.Context, q, t string, limit, offset int) ([]Symbol, error) {
	safe := utils.SanitizeFTS(q)
	if safe == "" {
		return nil, nil
	}
	sql := `SELECT symbols.name, symbols.type, symbols.package_path, symbols.receiver_type, symbols.signature, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.start_col, symbols.end_line, symbols.structural_hash, symbols.indegree, symbols.pagerank, symbols.churn_score, symbols.ai_summary
            FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid
            WHERE symbols_fts MATCH ?`
	args := []any{safe}
	if t != "" {
		sql += " AND symbols.type = ?"
		args = append(args, t)
	}
	if limit == 0 {
		limit = 500
	}
	sql += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search symbols failed: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, pagerank, churn_score, ai_summary
            FROM symbols
            WHERE name = ? AND path = ?`
	rows, err := s.query(ctx, sql, name, path)
	if err != nil {
		return nil, fmt.Errorf("get symbols by name in file failed: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) GetSymbolsByStructuralHash(ctx context.Context, hash string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, pagerank, churn_score, ai_summary
            FROM symbols
            WHERE structural_hash = ?`
	rows, err := s.query(ctx, sql, hash)
	if err != nil {
		return nil, fmt.Errorf("get symbols by structural hash failed: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}
func (s *Store) SearchSymbolsWeighted(ctx context.Context, q, t string) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		safe := utils.SanitizeFTS(q)
		if safe == "" {
			return
		}
		sql := `SELECT symbols.name, symbols.type, symbols.package_path, symbols.receiver_type, symbols.signature, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.start_col, symbols.end_line, symbols.structural_hash, bm25(symbols_fts, 10.0, 2.0, 5.0, 1.0, 0.5) as relevance, symbols.pagerank, symbols.churn_score, symbols.ai_summary
                FROM symbols JOIN symbols_fts ON symbols.id = symbols_fts.rowid
                WHERE symbols_fts MATCH ?`
		args := []any{safe}
		if t != "" {
			sql += " AND symbols.type = ?"
			args = append(args, t)
		}
		sql += " ORDER BY relevance ASC LIMIT 500"
		rows, err := s.query(ctx, sql, args...)
		if err != nil {
			yield(Symbol{}, fmt.Errorf("weighted search failed: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var sym Symbol
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
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
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, pagerank, churn_score, ai_summary
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
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
			return nil, fmt.Errorf("scan symbol range failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) GetSymbolsByType(ctx context.Context, symType string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, pagerank, churn_score, ai_summary
            FROM symbols
            WHERE type = ?`
	rows, err := s.query(ctx, sql, symType)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols by type: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
			return nil, fmt.Errorf("scan symbol by type failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *Store) GetInterfaces(ctx context.Context) ([]Symbol, error) {
	return s.GetSymbolsByType(ctx, "interface")
}

func (s *Store) GetAllSymbols(ctx context.Context) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		rows, err := s.query(ctx, "SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, pagerank, churn_score, ai_summary FROM symbols")
		if err != nil {
			yield(Symbol{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var sym Symbol
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {
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
func (s *Store) GetAllCalls(ctx context.Context) iter.Seq2[Call, error] {
	return func(yield func(Call, error) bool) {
		rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line, callee_path, link_type FROM calls")
		if err != nil {
			yield(Call{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var c Call
			if err := rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line, &c.CalleePath, &c.LinkType); err != nil {
				if !yield(Call{}, err) {
					return
				}
				continue
			}
			if !yield(c, nil) {
				return
			}
		}
	}
}

func (s *Store) GetAllFailedTests(ctx context.Context) iter.Seq2[types.TestResult, error] {
	return func(yield func(types.TestResult, error) bool) {
		rows, err := s.query(ctx, "SELECT test_name, status, error_message, stack_trace, target_symbol, duration_ms, project FROM test_results WHERE status = 'fail'")
		if err != nil {
			yield(types.TestResult{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var r types.TestResult
			if err := rows.Scan(&r.TestName, &r.Status, &r.ErrorMessage, &r.StackTrace, &r.TargetSymbol, &r.DurationMS, &r.Project); err != nil {
				if !yield(types.TestResult{}, err) {
					return
				}
				continue
			}
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (s *Store) UpdateSymbolCentrality(ctx context.Context, name, path string, centrality int) error {
	_, err := s.exec(ctx, "UPDATE symbols SET indegree = ? WHERE (name = ? OR (package_path || '.' || name) = ?) AND (path = ? OR ? = '')", centrality, name, name, path, path)
	if err != nil {
		return fmt.Errorf("failed to update centrality: %w", err)
	}
	return nil
}

func (s *Store) UpdateSymbolChurn(ctx context.Context, path string, score float64) error {
	_, err := s.exec(ctx, "UPDATE symbols SET churn_score = ? WHERE path = ?", score, path)
	if err != nil {
		return fmt.Errorf("failed to update churn score: %w", err)
	}
	return nil
}

func (s *Store) UpdateSymbolPageRank(ctx context.Context, name, path string, score float64) error {
	_, err := s.exec(ctx, "UPDATE symbols SET pagerank = ? WHERE (name = ? OR (package_path || '.' || name) = ?) AND (path = ? OR ? = '')", score, name, name, path, path)
	if err != nil {
		return fmt.Errorf("failed to update pagerank: %w", err)
	}
	return nil
}

func (s *Store) ExportDelta(ctx context.Context, syncDir string) error {
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		return err
	}

	paths, err := s.GetAllFilePaths(ctx)
	if err != nil {
		return err
	}

	for _, p := range paths {
		idx, err := s.GetFileIndex(ctx, p)
		if err != nil { continue }

		symbols, _ := s.GetSymbolsByRange(ctx, p, 0, 999999)
		
		rows, _ := s.query(ctx, "SELECT caller_name, callee_name, path, line, callee_path, link_type FROM calls WHERE path = ?", p)
		var calls []Call
		if rows != nil {
			for rows.Next() {
				var c Call
				rows.Scan(&c.CallerName, &c.CalleeName, &c.Path, &c.Line, &c.CalleePath, &c.LinkType)
				calls = append(calls, c)
			}
			rows.Close()
		}

		delta := SovereignDelta{
			Path:    p,
			Hash:    idx.Hash,
			Symbols: symbols,
			Calls:   calls,
		}

		// Use a sanitized path for the filename
		relPath, _ := filepath.Rel("/", p)
		if relPath == "" { relPath = filepath.Base(p) }
		syncFile := filepath.Join(syncDir, strings.ReplaceAll(relPath, "/", "_")+".json")
		
		data, _ := json.MarshalIndent(delta, "", "  ")
		os.WriteFile(syncFile, data, 0644)
	}
	return nil
}

func (s *Store) ImportDelta(ctx context.Context, syncDir string) error {
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Repository) error {
		var walk func(string) error
		walk = func(dir string) error {
			entries, err := os.ReadDir(dir)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				select {
				case <-txCtx.Done():
					return txCtx.Err()
				default:
				}

				fullPath := filepath.Join(dir, entry.Name())
				if entry.IsDir() {
					if err := walk(fullPath); err != nil {
						return err
					}
					continue
				}

				if filepath.Ext(fullPath) != ".json" {
					continue
				}

				data, err := os.ReadFile(fullPath)
				if err != nil {
					continue
				}

				var delta SovereignDelta
				if err := json.Unmarshal(data, &delta); err != nil {
					continue
				}

				tx.SaveFileIndex(txCtx, &FileIndex{
					Path:    delta.Path,
					Hash:    delta.Hash,
					ASTJSON: "{}", // Not needed for Delta Sync
				})
				tx.ClearSymbols(txCtx, delta.Path)
				tx.ClearCalls(txCtx, delta.Path)
				for _, sym := range delta.Symbols {
					tx.SaveSymbol(txCtx, &sym)
				}
				for _, call := range delta.Calls {
					tx.SaveCall(txCtx, call)
				}
			}
			return nil
		}
		return walk(syncDir)
	})
}

func (s *Store) SaveCall(ctx context.Context, c Call) error {
	if c.LinkType == "" {
		c.LinkType = "call"
	}
	_, err := s.exec(ctx, "INSERT OR IGNORE INTO calls (caller_name, callee_name, path, line, callee_path, link_type) VALUES (?, ?, ?, ?, ?, ?)", c.CallerName, c.CalleeName, c.Path, c.Line, c.CalleePath, c.LinkType)
	if err != nil {
		return fmt.Errorf("failed to save call: %w", err)
	}

	// Increment indegree for the callee symbol
	_, err = s.exec(ctx, "UPDATE symbols SET indegree = indegree + 1 WHERE name = ? AND (path = ? OR ? = '')", c.CalleeName, c.CalleePath, c.CalleePath)
	if err != nil {
		// Log but don't fail, as symbol might not be indexed yet
		return nil
	}

	return nil
}


func (s *Store) GetCallers(ctx context.Context, callee string, limit, offset int) ([]Call, error) {
	if limit == 0 {
		limit = 500
	}
	rows, err := s.query(ctx, "SELECT caller_name, callee_name, path, line, callee_path, link_type FROM calls WHERE callee_name = ? LIMIT ? OFFSET ?", callee, limit, offset)
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

func (s *Store) GetCallersRecursive(ctx context.Context, symbol string, path string, maxDepth int) ([]Call, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	query := `
	WITH RECURSIVE blast_radius(caller_name, caller_path, distance, link_type, path_trace) AS (
		SELECT caller_name, path, 1, link_type, ',' || caller_name || ':' || path || ','
		FROM calls
		WHERE (callee_name = ? OR callee_name = (SELECT CASE WHEN package_path != '' THEN package_path || '.' || name ELSE name END FROM symbols WHERE name = ? AND path = ? LIMIT 1))
		  AND (callee_path = ? OR callee_path = '')
		
		UNION
		
		SELECT c.caller_name, c.path, br.distance + 1, c.link_type, br.path_trace || c.caller_name || ':' || c.path || ','
		FROM calls c
		JOIN blast_radius br ON c.callee_name = br.caller_name AND (c.callee_path = br.caller_path OR c.callee_path = '')
		WHERE br.distance < ? AND br.path_trace NOT LIKE '%,' || c.caller_name || ':' || c.path || ',%'
	)
	SELECT DISTINCT caller_name, caller_path, distance, link_type 
	FROM blast_radius 
	ORDER BY distance ASC LIMIT 500;`

	rows, err := s.query(ctx, query, symbol, symbol, path, path, maxDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Call
	for rows.Next() {
		var c Call
		var dist int
		if err := rows.Scan(&c.CallerName, &c.Path, &dist, &c.LinkType); err != nil {
			return nil, err
		}
		// We use Line field to store distance for now to avoid breaking Call struct or creating new one
		// In a real refactor, we might want a separate struct.
		c.Line = dist 
		res = append(res, c)
	}
	return res, nil
}

func (s *Store) GetAffectedTestsRecursive(ctx context.Context, symbol, path string) ([]Symbol, error) {
	query := `
	WITH RECURSIVE affected(name, path, distance) AS (
		SELECT CASE WHEN package_path != '' THEN package_path || '.' || name ELSE name END, path, 0
		FROM symbols
		WHERE (name = ? OR (CASE WHEN package_path != '' THEN package_path || '.' || name ELSE name END) = ?) AND path = ?

		UNION

		SELECT c.caller_name, c.path, a.distance + 1
		FROM calls c
		JOIN affected a ON c.callee_name = a.name AND (c.callee_path = a.path OR c.callee_path = '')
		WHERE a.distance < 10 AND c.caller_name NOT IN ('main', 'init')
	)
	SELECT DISTINCT s.name, s.type, s.package_path, s.receiver_type, s.signature, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.start_col, s.end_line, s.structural_hash
	FROM symbols s
	JOIN affected a ON (CASE WHEN s.package_path != '' THEN s.package_path || '.' || s.name ELSE s.name END) = a.name AND s.path = a.path
	WHERE (s.name LIKE 'Test%' AND (s.type = 'function' OR s.type = 'method'))
	   OR s.path LIKE '%_test.go'
	ORDER BY s.path, s.start_line;`
	rows, err := s.query(ctx, query, symbol, symbol, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash); err != nil {
			return nil, err
		}
		res = append(res, sym)
	}
	return res, nil
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
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, end_line 
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
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("scan unused symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
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

func (s *Store) SaveViolation(ctx context.Context, v *types.ASTRuleMatch) error {
	query := `INSERT INTO violations (rule_id, file_path, message, severity, start_line, start_col, text) 
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.exec(ctx, query, v.RuleID, v.File, v.Message, v.Severity, v.Range.Start.Line, v.Range.Start.Column, v.Text)
	if err != nil {
		return fmt.Errorf("failed to save violation: %w", err)
	}
	return nil
}

func (s *Store) GetViolationsByFile(ctx context.Context, path string) ([]Violation, error) {
	rows, err := s.query(ctx, "SELECT id, rule_id, file_path, message, severity, start_line, start_col, text FROM violations WHERE file_path = ?", path)
	if err != nil {
		return nil, fmt.Errorf("failed to get violations: %w", err)
	}
	defer rows.Close()

	var res []Violation
	for rows.Next() {
		var v Violation
		if err := rows.Scan(&v.ID, &v.RuleID, &v.FilePath, &v.Message, &v.Severity, &v.StartLine, &v.StartCol, &v.Text); err != nil {
			return nil, fmt.Errorf("scan violation failed: %w", err)
		}
		res = append(res, v)
	}
	return res, nil
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
