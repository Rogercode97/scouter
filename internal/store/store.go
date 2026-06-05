package store

import (
	_ "embed"
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
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)




type Flow struct {
	Source string `json:"source"`
	Sink   string `json:"sink"`
	Type   string `json:"type"` // assignment, parameter, return
	Path   string `json:"path"`
	Line   int    `json:"line"`
}

// SovereignDelta represents the index data for a single file, stable for Git.
type SovereignDelta struct {
	Path    string   `json:"path"`
	Hash    string   `json:"hash"`
	Symbols []Symbol `json:"symbols"`
	Calls   []Call   `json:"calls"`
	Flows   []Flow   `json:"flows"`
}

type BatchItem struct {
	Index      *FileIndex
	Symbols    []Symbol
	Calls      []Call
	Flows      []Flow
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

//go:embed schema.sql
var schemaSQL string

type storeImpl struct {
	dbRead  *sql.DB
	dbWrite *sql.DB
	tx      *sql.Tx
}

var _ Store = (*storeImpl)(nil)

func NewStore(ctx context.Context, dbPath string) (Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	var dbRead, dbWrite *sql.DB
	var err error

	if strings.Contains(dbPath, ":memory:") {
		// For in-memory databases, we must share the exact same connection pool
		// between read and write to prevent "no such table" isolation errors.
		dbWrite, err = sql.Open("sqlite3", ":memory:")
		if err != nil {
			return nil, fmt.Errorf("failed to open memory database: %w", err)
		}
		dbWrite.SetMaxOpenConns(1)
		dbRead = dbWrite
	} else {
		// Optimized SQLite parameters for high-performance bulk indexing
		separator := "?"
		if strings.Contains(dbPath, "?") {
			separator = "&"
		}
		dsn := dbPath
		if !strings.HasPrefix(dsn, "file:") {
			dsn = "file:" + dsn
		}
		
		// Write pool
		dsnWrite := fmt.Sprintf("%s%s_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-10000)&_txlock=immediate", dsn, separator)
		dbWrite, err = sql.Open("sqlite3", dsnWrite)
		if err != nil {
			return nil, fmt.Errorf("failed to open write database: %w", err)
		}
		dbWrite.SetMaxOpenConns(1)

		// Read pool
		dsnRead := fmt.Sprintf("%s%s_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-10000)", dsn, separator)
		dbRead, err = sql.Open("sqlite3", dsnRead)
		if err != nil {
			dbWrite.Close()
			return nil, fmt.Errorf("failed to open read database: %w", err)
		}
		dbRead.SetMaxOpenConns(runtime.NumCPU() * 2)
	}

	tx, err := dbWrite.BeginTx(ctx, nil)
	if err != nil {
		dbRead.Close()
		dbWrite.Close()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := migrate(ctx, tx); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit migration: %w", err)
	}

	s := &storeImpl{
		dbRead:  dbRead,
		dbWrite: dbWrite,
	}

	// Go 1.25 native cleanup
	type dbState struct{ r, w *sql.DB }
	runtime.AddCleanup(s, func(state dbState) {
		_ = state.r.Close()
		_ = state.w.Close()
	}, dbState{dbRead, dbWrite})

	return s, nil
}

func migrate(ctx context.Context, tx *sql.Tx) error {
	queries := strings.Split(schemaSQL, "\n\n")
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" { continue }
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
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

func (s *storeImpl) exec(ctx context.Context, q string, a ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, q, a...)
	}
	return s.dbWrite.ExecContext(ctx, q, a...)
}

func (s *storeImpl) queryRow(ctx context.Context, q string, a ...any) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, q, a...)
	}
	return s.dbRead.QueryRowContext(ctx, q, a...)
}

func (s *storeImpl) query(ctx context.Context, q string, a ...any) (*sql.Rows, error) {
	if s.tx != nil {
		return s.tx.QueryContext(ctx, q, a...)
	}
	return s.dbRead.QueryContext(ctx, q, a...)
}

func (s *storeImpl) GetFileIndex(ctx context.Context, p string) (*FileIndex, error) {
	var idx FileIndex
	err := s.queryRow(ctx, "SELECT path, mtime, hash, ast_json, project, freshness FROM file_index WHERE path = ?", p).Scan(&idx.Path, &idx.Mtime, &idx.Hash, &idx.AstJson, &idx.Project, &idx.Freshness)
	if err != nil {
		return nil, fmt.Errorf("failed to get file index: %w", err)
	}
	return &idx, nil
}

func (s *storeImpl) SaveFileIndex(ctx context.Context, idx *FileIndex) error {
	query := `INSERT INTO file_index (path, mtime, hash, ast_json, project, freshness)
              VALUES (?, ?, ?, ?, ?, ?)
              ON CONFLICT(path) DO UPDATE SET
                mtime=excluded.mtime,
                hash=excluded.hash,
                ast_json=excluded.ast_json,
                project=excluded.project,
                freshness=excluded.freshness`
	_, err := s.exec(ctx, query, idx.Path, idx.Mtime, idx.Hash, idx.AstJson, idx.Project, idx.Freshness)
	if err != nil {
		return fmt.Errorf("failed to save file index: %w", err)
	}
	return nil
}

func (s *storeImpl) SaveFileIndexBatch(ctx context.Context, items []BatchItem) error {
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Store) error {
		var allSymbols []*Symbol
		var allCalls []Call
		for i := range items {
			item := &items[i]
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
				if err := tx.ClearFlows(txCtx, item.Index.Path); err != nil {
					return fmt.Errorf("failed to clear flows for %s: %w", item.Index.Path, err)
				}
			}
			for j := range item.Symbols {
				allSymbols = append(allSymbols, &item.Symbols[j])
			}
			for _, call := range item.Calls {
				allCalls = append(allCalls, call)
			}
			for _, flow := range item.Flows {
				if err := tx.SaveFlow(txCtx, flow); err != nil {
					return fmt.Errorf("failed to save flow: %w", err)
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
		
		if err := tx.SaveSymbolsBulk(txCtx, allSymbols); err != nil {
			return fmt.Errorf("failed to save symbols bulk: %w", err)
		}
		if err := tx.SaveCallsBulk(txCtx, allCalls); err != nil {
			return fmt.Errorf("failed to save calls bulk: %w", err)
		}

		return nil
	})
}

func (s *storeImpl) GetDirectoryHash(ctx context.Context, p string) (string, int64, error) {
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

func (s *storeImpl) SaveDirectoryHash(ctx context.Context, p string, hash string, mtime int64) error {
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

func (s *storeImpl) ClearSymbols(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM symbols WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to clear symbols: %w", err)
	}
	return nil
}

func (s *storeImpl) SaveSymbol(ctx context.Context, sym *Symbol) error {

	metricsJSON := "{}"
	if sym.Metrics != nil {
		if b, err := json.Marshal(sym.Metrics); err == nil {
			metricsJSON = string(b)
		}
	}
	res, err := s.exec(ctx, "INSERT INTO symbols (name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.Indegree, sym.Relevance, sym.Pagerank, sym.ChurnScore, sym.RuntimeHits, sym.AiSummary, metricsJSON)
	if err != nil {
		return fmt.Errorf("failed to save symbol: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		sym.ID = int(id)
	}
	return nil
}

func (s *storeImpl) SaveSymbolsBulk(ctx context.Context, symbols []*Symbol) error {
	if len(symbols) == 0 {
		return nil
	}
	batchSize := 20 // 20 parameters per symbol, max 500 params per query
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]

		query := "INSERT INTO symbols (name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json) VALUES "
		var args []any
		var placeholders []string
		for _, sym := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			metricsJSON := "{}"
			if sym.Metrics != nil {
				if b, err := json.Marshal(sym.Metrics); err == nil {
					metricsJSON = string(b)
				}
			}
			args = append(args, sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.Indegree, sym.Relevance, sym.Pagerank, sym.ChurnScore, sym.RuntimeHits, sym.AiSummary, metricsJSON)
		}
		query += strings.Join(placeholders, ", ") // #nosec G202 + " RETURNING id"
		rows, err := s.query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to save symbols bulk: %w", err)
		}
		
		idx := 0
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan returned id: %w", err)
			}
			if idx < len(batch) {
				batch[idx].ID = id
			}
			idx++
		}
		rows.Close()
	}
	return nil
}

func (s *storeImpl) SearchSymbols(ctx context.Context, q, t string, limit, offset int) ([]Symbol, error) {
	safe := utils.SanitizeFTS(q)
	if safe == "" {
		return nil, nil
	}
	sql := `SELECT symbols.name, symbols.type, symbols.package_path, symbols.receiver_type, symbols.signature, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.start_col, symbols.end_line, symbols.structural_hash, symbols.indegree, symbols.relevance, symbols.pagerank, symbols.churn_score, symbols.runtime_hits, symbols.ai_summary, symbols.metrics_json
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
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *storeImpl) GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json
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
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *storeImpl) UpdateSymbolRuntimeHits(ctx context.Context, name, path string, hits int) error {
	sql := `UPDATE symbols SET runtime_hits = ? WHERE name = ? AND path = ?`
	_, err := s.exec(ctx, sql, hits, name, path)
	if err != nil {
		return fmt.Errorf("update symbol runtime hits failed: %w", err)
	}
	return nil
}

func (s *storeImpl) GetSymbolsByStructuralHash(ctx context.Context, hash string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json
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
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}
func (s *storeImpl) SearchSymbolsWeighted(ctx context.Context, q, t string) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		safe := utils.SanitizeFTS(q)
		if safe == "" {
			return
		}
		sql := `SELECT symbols.name, symbols.type, symbols.package_path, symbols.receiver_type, symbols.signature, symbols.doc, symbols.path, symbols.start_byte, symbols.end_byte, symbols.start_line, symbols.start_col, symbols.end_line, symbols.structural_hash, symbols.indegree, bm25(symbols_fts, 10.0, 2.0, 5.0, 1.0, 0.5) as relevance, symbols.pagerank, symbols.churn_score, symbols.runtime_hits, symbols.ai_summary, symbols.metrics_json
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
			var metricsJSON string
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
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

func (s *storeImpl) GetSymbolsByRange(ctx context.Context, path string, start, end int) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json
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
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol range failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}
func (s *storeImpl) GetSymbolsByPathPrefix(ctx context.Context, pathPrefix string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json
            FROM symbols
            WHERE path LIKE ? ORDER BY path ASC, start_line ASC`
	
	// Add % to match any file that starts with the prefix (e.g., directory or exact file)
	likePattern := pathPrefix
	if !strings.HasSuffix(likePattern, "%") {
		likePattern += "%"
	}

	rows, err := s.query(ctx, sql, likePattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols by path prefix: %w", err)
	}
	defer rows.Close()
	var res []Symbol
	for rows.Next() {
		var sym Symbol
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol by path prefix failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *storeImpl) GetSymbolsByType(ctx context.Context, symType string) ([]Symbol, error) {
	sql := `SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json
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
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol by type failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}

func (s *storeImpl) GetInterfaces(ctx context.Context) ([]Symbol, error) {
	return s.GetSymbolsByType(ctx, "interface")
}

func (s *storeImpl) GetAllSymbols(ctx context.Context) iter.Seq2[Symbol, error] {
	return func(yield func(Symbol, error) bool) {
		rows, err := s.query(ctx, "SELECT name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, indegree, relevance, pagerank, churn_score, runtime_hits, ai_summary, metrics_json FROM symbols")
		if err != nil {
			yield(Symbol{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var sym Symbol
			var metricsJSON string
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.Relevance, &sym.Pagerank, &sym.ChurnScore, &sym.RuntimeHits, &sym.AiSummary, &metricsJSON); err != nil {
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
func (s *storeImpl) GetAllCalls(ctx context.Context) iter.Seq2[Call, error] {
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

func (s *storeImpl) GetAllFailedTests(ctx context.Context) iter.Seq2[types.TestResult, error] {
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

func (s *storeImpl) UpdateSymbolCentrality(ctx context.Context, name, path string, centrality int) error {
	_, err := s.exec(ctx, "UPDATE symbols SET indegree = ? WHERE (name = ? OR (package_path || '.' || name) = ?) AND (path = ? OR ? = '')", centrality, name, name, path, path)
	if err != nil {
		return fmt.Errorf("failed to update centrality: %w", err)
	}
	return nil
}

func (s *storeImpl) RecomputeIndegrees(ctx context.Context) error {
	_, err := s.exec(ctx, "UPDATE symbols SET indegree = (SELECT count(*) FROM calls WHERE calls.callee_name = symbols.name)")
	if err != nil {
		return fmt.Errorf("failed to recompute indegrees: %w", err)
	}
	return nil
}

func (s *storeImpl) UpdateSymbolChurn(ctx context.Context, path string, score float64) error {
	_, err := s.exec(ctx, "UPDATE symbols SET churn_score = ? WHERE path = ?", score, path)
	if err != nil {
		return fmt.Errorf("failed to update churn score: %w", err)
	}
	return nil
}

func (s *storeImpl) UpdateSymbolPagerank(ctx context.Context, name, path string, score float64) error {
	_, err := s.exec(ctx, "UPDATE symbols SET pagerank = ? WHERE name = ? AND path = ?", score, name, path)
	if err != nil {
		return fmt.Errorf("failed to update pagerank: %w", err)
	}
	return nil
}

func (s *storeImpl) RecordSymbolUsage(ctx context.Context, env string, usages []UsageRecord) error {
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Store) error {
		txImpl := tx.(*storeImpl)
		// We use a prepared statement inside the transaction for efficiency
		stmt, err := txImpl.tx.PrepareContext(txCtx, `
			INSERT INTO symbol_usage (symbol_id, environment, last_used, hit_count)
			SELECT id, ?, ?, ?
			FROM symbols
			WHERE (name = ? OR (package_path || '.' || name) = ?) AND (path = ? OR ? = '')
			ON CONFLICT(symbol_id, environment) DO UPDATE SET
				hit_count = hit_count + excluded.hit_count,
				last_used = MAX(last_used, excluded.last_used)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, u := range usages {
			if _, err := stmt.ExecContext(txCtx, env, u.LastUsed, u.HitCount, u.SymbolName, u.SymbolName, u.SymbolPath, u.SymbolPath); err != nil {
				return fmt.Errorf("failed to record usage for %s: %w", u.SymbolName, err)
			}
		}
		return nil
	})
}

func (s *storeImpl) ExportDelta(ctx context.Context, syncDir string) error {
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

func (s *storeImpl) ImportDelta(ctx context.Context, syncDir string) error {
	return s.WithTransaction(ctx, func(txCtx context.Context, tx Store) error {
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
					AstJson: "{}", // Not needed for Delta Sync
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

func (s *storeImpl) SaveCall(ctx context.Context, c Call) error {
	if c.LinkType == "" {
		c.LinkType = "call"
	}
	_, err := s.exec(ctx, "INSERT OR IGNORE INTO calls (caller_name, callee_name, path, line, callee_path, link_type) VALUES (?, ?, ?, ?, ?, ?)", c.CallerName, c.CalleeName, c.Path, c.Line, c.CalleePath, c.LinkType)
	if err != nil {
		return fmt.Errorf("failed to save call: %w", err)
	}
	return nil
}

func (s *storeImpl) SaveCallsBulk(ctx context.Context, calls []Call) error {
	if len(calls) == 0 {
		return nil
	}
	batchSize := 80 // 6 parameters per call, max 500 params per query
	for i := 0; i < len(calls); i += batchSize {
		end := i + batchSize
		if end > len(calls) {
			end = len(calls)
		}
		batch := calls[i:end]

		query := "INSERT OR IGNORE INTO calls (caller_name, callee_name, path, line, callee_path, link_type) VALUES "
		var args []any
		var placeholders []string
		for _, c := range batch {
			if c.LinkType == "" {
				c.LinkType = "call"
			}
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
			args = append(args, c.CallerName, c.CalleeName, c.Path, c.Line, c.CalleePath, c.LinkType)
		}
		query += strings.Join(placeholders, ", ") // #nosec G202
		if _, err := s.exec(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to save calls bulk: %w", err)
		}
	}
	return nil
}

func (s *storeImpl) SaveFlow(ctx context.Context, flow Flow) error {
	query := `INSERT INTO flows (source, sink, type, path, line) VALUES (?, ?, ?, ?, ?)`
	_, err := s.exec(ctx, query, flow.Source, flow.Sink, flow.Type, flow.Path, flow.Line)
	if err != nil {
		return fmt.Errorf("failed to save flow: %w", err)
	}
	return nil
}

func (s *storeImpl) GetFlows(ctx context.Context, sink string) ([]Flow, error) {
	rows, err := s.query(ctx, "SELECT source, sink, type, path, line FROM flows WHERE sink = ?", sink)
	if err != nil {
		return nil, fmt.Errorf("failed to get flows: %w", err)
	}
	defer rows.Close()

	var res []Flow
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.Source, &f.Sink, &f.Type, &f.Path, &f.Line); err != nil {
			return nil, fmt.Errorf("scan flow failed: %w", err)
		}
		res = append(res, f)
	}
	return res, nil
}

func (s *storeImpl) ClearFlows(ctx context.Context, path string) error {
	_, err := s.exec(ctx, "DELETE FROM flows WHERE path = ?", path)
	return err
}


func (s *storeImpl) GetCallers(ctx context.Context, callee string, limit, offset int) ([]Call, error) {
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

func (s *storeImpl) GetCallees(ctx context.Context, caller string) ([]Call, error) {
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

func (s *storeImpl) GetCallersRecursive(ctx context.Context, symbol string, path string, maxDepth int) ([]Call, error) {
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
	ORDER BY distance ASC LIMIT 500`

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

func (s *storeImpl) GetAffectedTestsRecursive(ctx context.Context, symbol, path string) ([]Symbol, error) {
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
	ORDER BY s.path, s.start_line`
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

func (s *storeImpl) ClearCalls(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM calls WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to clear calls: %w", err)
	}
	return nil
}

func (s *storeImpl) SaveDependency(ctx context.Context, d *types.Dependency) error {
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

func (s *storeImpl) GetDependencies(ctx context.Context) ([]types.Dependency, error) {
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

func (s *storeImpl) ClearDependencies(ctx context.Context) error {
	_, err := s.exec(ctx, "DELETE FROM dependencies")
	if err != nil {
		return fmt.Errorf("failed to clear dependencies: %w", err)
	}
	return nil
}

func (s *storeImpl) GetUnusedSymbols(ctx context.Context, exp bool) ([]Symbol, error) {
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

func (s *storeImpl) SaveTestResult(ctx context.Context, r *types.TestResult) error {
	_, err := s.exec(ctx, "INSERT INTO test_results (test_name, status, error_message, stack_trace, target_symbol, duration_ms, project) VALUES (?, ?, ?, ?, ?, ?, ?)", r.TestName, r.Status, r.ErrorMessage, r.StackTrace, r.TargetSymbol, r.DurationMS, r.Project)
	if err != nil {
		return fmt.Errorf("failed to save test result: %w", err)
	}
	return nil
}

func (s *storeImpl) GetHealthReport(ctx context.Context, sym string, fails bool) iter.Seq2[types.TestResult, error] {
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

func (s *storeImpl) ClearTestResults(ctx context.Context) error {
	_, err := s.exec(ctx, "DELETE FROM test_results")
	if err != nil {
		return fmt.Errorf("failed to clear test results: %w", err)
	}
	return nil
}

func (s *storeImpl) GetStats(ctx context.Context) (int, int, error) {
	var fc, sc int
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM file_index").Scan(&fc); err != nil {
		return 0, 0, fmt.Errorf("failed to get file count: %w", err)
	}
	if err := s.queryRow(ctx, "SELECT COUNT(*) FROM symbols").Scan(&sc); err != nil {
		return 0, 0, fmt.Errorf("failed to get symbol count: %w", err)
	}
	return fc, sc, nil
}

func (s *storeImpl) GetAllFilePaths(ctx context.Context) ([]string, error) {
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

func (s *storeImpl) DeleteFileIndex(ctx context.Context, p string) error {
	_, err := s.exec(ctx, "DELETE FROM file_index WHERE path = ?", p)
	if err != nil {
		return fmt.Errorf("failed to delete file index: %w", err)
	}
	return nil
}

func (s *storeImpl) SaveViolation(ctx context.Context, v *types.ASTRuleMatch) error {
	query := `INSERT INTO violations (rule_id, file_path, message, severity, start_line, start_col, text) 
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.exec(ctx, query, v.RuleID, v.File, v.Message, v.Severity, v.Range.Start.Line, v.Range.Start.Column, v.Text)
	if err != nil {
		return fmt.Errorf("failed to save violation: %w", err)
	}
	return nil
}

func (s *storeImpl) GetViolationsByFile(ctx context.Context, path string) ([]Violation, error) {
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

func (s *storeImpl) WithTransaction(ctx context.Context, fn func(context.Context, Store) error) error {
	tx, err := s.dbWrite.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(ctx, &storeImpl{dbRead: s.dbRead, dbWrite: s.dbWrite, tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *storeImpl) Close() error { 
	s.dbRead.Close()
	return s.dbWrite.Close() 
}

type CriticalSymbol struct {
	Symbol
	Centrality int `json:"centrality"`
	Fragility  int `json:"fragility"`
}

func (s *storeImpl) InsertSemanticVector(ctx context.Context, symbolID int64, embedding []float32) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	// Insert into vec_ast_meta first to get rowid
	res, err := s.exec(ctx, "INSERT INTO vec_ast_meta (symbol_id) VALUES (?) ON CONFLICT(symbol_id) DO NOTHING", symbolID)
	if err != nil {
		return fmt.Errorf("failed to insert vec_ast_meta: %w", err)
	}

	var rowID int64
	// If ON CONFLICT DO NOTHING triggered, res.LastInsertId() might be 0.
	// We should probably just select it if it exists.
	rowID, err = res.LastInsertId()
	if err != nil || rowID == 0 {
		err = s.queryRow(ctx, "SELECT rowid FROM vec_ast_meta WHERE symbol_id = ?", symbolID).Scan(&rowID)
		if err != nil {
			return fmt.Errorf("failed to get rowid for symbol %d: %w", symbolID, err)
		}
	}

	// Now insert/update vec_ast
	_, err = s.exec(ctx, "INSERT OR REPLACE INTO vec_ast (rowid, embedding) VALUES (?, ?)", rowID, string(embJSON))
	if err != nil {
		return fmt.Errorf("failed to insert vec_ast: %w", err)
	}

	return nil
}

func (s *storeImpl) SearchSemantic(ctx context.Context, queryEmbedding []float32, limit int) ([]Symbol, error) {
        embJSON, err := json.Marshal(queryEmbedding)
        if err != nil {
                return nil, fmt.Errorf("failed to marshal embedding: %w", err)
        }

        // According to sqlite-vec documentation, we can query like this:
        // SELECT m.symbol_id, v.distance FROM vec_ast v JOIN vec_ast_meta m ON v.rowid = m.rowid WHERE v.embedding MATCH ? ORDER BY distance LIMIT ?
        // Or using `AND k = ?` syntax depending on sqlite-vec versions. Let's try `AND k = ?` as it limits KNN.
        // Or simply `LIMIT ?` at the end of the query.
        
        query := `
        SELECT s.name, s.type, s.package_path, s.receiver_type, s.signature, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
        FROM vec_ast v
        JOIN vec_ast_meta m ON v.rowid = m.rowid
        JOIN symbols s ON s.id = m.symbol_id
        WHERE v.embedding MATCH ? AND k = ?
        ORDER BY distance
        `
        
        rows, err := s.query(ctx, query, string(embJSON), limit)
        if err != nil {
                return nil, fmt.Errorf("failed to execute semantic search query: %w", err)
        }
        defer rows.Close()

        var res []Symbol
        for rows.Next() {
                var sym Symbol
                if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
                        return nil, fmt.Errorf("scan semantic symbol failed: %w", err)
                }
                res = append(res, sym)
        }
        return res, nil
}
func (s *storeImpl) UpdateSymbolPageranksBulk(ctx context.Context, updates map[string]float64) error {
	if len(updates) == 0 {
		return nil
	}

	return s.WithTransaction(ctx, func(txCtx context.Context, tx Store) error {
		txImpl := tx.(*storeImpl)

		_, err := txImpl.tx.ExecContext(txCtx, "CREATE TEMP TABLE IF NOT EXISTS pr_updates (name TEXT, path TEXT, pr REAL)")
		if err != nil {
			return fmt.Errorf("failed to create temp table pr_updates: %w", err)
		}
		defer txImpl.tx.ExecContext(context.Background(), "DROP TABLE IF EXISTS pr_updates")

		type entry struct {
			Name string
			Path string
			PR   float64
		}
		var entries []entry
		for k, v := range updates {
			parts := strings.SplitN(k, ":", 2)
			if len(parts) == 2 {
				entries = append(entries, entry{Name: parts[0], Path: parts[1], PR: v})
			}
		}

		batchSize := 100
		for i := 0; i < len(entries); i += batchSize {
			end := i + batchSize
			if end > len(entries) {
				end = len(entries)
			}
			chunk := entries[i:end]

			query := "INSERT INTO pr_updates (name, path, pr) VALUES "
			var args []interface{}
			var placeholders []string
			for _, e := range chunk {
				placeholders = append(placeholders, "(?, ?, ?)")
				args = append(args, e.Name, e.Path, e.PR)
			}
			query += strings.Join(placeholders, ", ") // #nosec G202

			_, err = txImpl.tx.ExecContext(txCtx, query, args...)
			if err != nil {
				return fmt.Errorf("failed to batch insert pr_updates: %w", err)
			}
		}

		updateQuery := `
			UPDATE symbols 
			SET pagerank = (SELECT pr FROM pr_updates WHERE pr_updates.name = symbols.name AND pr_updates.path = symbols.path) 
			WHERE EXISTS (SELECT 1 FROM pr_updates WHERE pr_updates.name = symbols.name AND pr_updates.path = symbols.path)
		`
		_, err = txImpl.tx.ExecContext(txCtx, updateQuery)
		if err != nil {
			return fmt.Errorf("failed to update pageranks from temp table: %w", err)
		}

		return nil
	})
}

func (s *storeImpl) UpdateSymbolCentralitiesBulk(ctx context.Context, updates map[string]int) error {
	if len(updates) == 0 {
		return nil
	}

	return s.WithTransaction(ctx, func(txCtx context.Context, tx Store) error {
		txImpl := tx.(*storeImpl)

		_, err := txImpl.tx.ExecContext(txCtx, "CREATE TEMP TABLE IF NOT EXISTS cent_updates (name TEXT, path TEXT, centrality INTEGER)")
		if err != nil {
			return fmt.Errorf("failed to create temp table cent_updates: %w", err)
		}
		defer txImpl.tx.ExecContext(context.Background(), "DROP TABLE IF EXISTS cent_updates")

		type entry struct {
			Name       string
			Path       string
			Centrality int
		}
		var entries []entry
		for k, v := range updates {
			parts := strings.SplitN(k, ":", 2)
			if len(parts) == 2 {
				entries = append(entries, entry{Name: parts[0], Path: parts[1], Centrality: v})
			} else {
				entries = append(entries, entry{Name: k, Path: "", Centrality: v})
			}
		}

		batchSize := 100
		for i := 0; i < len(entries); i += batchSize {
			end := i + batchSize
			if end > len(entries) {
				end = len(entries)
			}
			chunk := entries[i:end]

			query := "INSERT INTO cent_updates (name, path, centrality) VALUES "
			var args []interface{}
			var placeholders []string
			for _, e := range chunk {
				placeholders = append(placeholders, "(?, ?, ?)")
				args = append(args, e.Name, e.Path, e.Centrality)
			}
			query += strings.Join(placeholders, ", ") // #nosec G202

			_, err = txImpl.tx.ExecContext(txCtx, query, args...)
			if err != nil {
				return fmt.Errorf("failed to batch insert cent_updates: %w", err)
			}
		}

		updateQuery := `
			UPDATE symbols 
			SET indegree = (SELECT centrality FROM cent_updates WHERE cent_updates.name = symbols.name AND cent_updates.path = symbols.path) 
			WHERE EXISTS (SELECT 1 FROM cent_updates WHERE cent_updates.name = symbols.name AND cent_updates.path = symbols.path)
		`
		_, err = txImpl.tx.ExecContext(txCtx, updateQuery)
		if err != nil {
			return fmt.Errorf("failed to update centralities from temp table: %w", err)
		}

		return nil
	})
}
