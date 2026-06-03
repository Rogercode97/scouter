package engram

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)

/**
 * ⚔️ HAKAISHIN SQLITE ADAPTER (WAVE 7)
 */
type SQLiteMemoryProvider struct {
	dbPath         string
	symbolSearcher func(ctx context.Context, query string) ([]types.Symbol, error)
}

func NewSQLiteMemoryProvider(dbPath string) *SQLiteMemoryProvider {
	return &SQLiteMemoryProvider{
		dbPath: dbPath,
	}
}

// DiscoverDBPath locates the Engram SQLite database.
// It prioritizes ENGRAM_DB_PATH env var, then defaults to ~/.engram/observations.db.
func DiscoverDBPath() (string, error) {
	if path := os.Getenv("ENGRAM_DB_PATH"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".engram", "observations.db"), nil
}

func (p *SQLiteMemoryProvider) WithSymbolSearcher(fn func(ctx context.Context, query string) ([]types.Symbol, error)) *SQLiteMemoryProvider {
	p.symbolSearcher = fn
	return p
}

func (p *SQLiteMemoryProvider) openDB(ctx context.Context, readOnly bool) (*sql.DB, error) {
	dsn := p.dbPath
	if readOnly {
		dsn = fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", p.dbPath)
	} else {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", p.dbPath)
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if !readOnly {
		db.SetMaxOpenConns(1) // Single writer pattern for SQLite
	}

	return db, nil
}

func (p *SQLiteMemoryProvider) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	db, err := p.openDB(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to open database (ro): %w", err)
	}
	defer db.Close()

	// Correct column names: 'project' and 'created_at' based on Engram schema.
	query := `
		SELECT content
		FROM observations
		WHERE project = ?
		  AND created_at >= datetime('now', ?)
		ORDER BY created_at DESC
	`
	
	timeModifier := fmt.Sprintf("-%d hours", hours)
	
	rows, err := db.QueryContext(ctx, query, project, timeModifier)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var observations []memory.Observation
	for rows.Next() {
		var obs memory.Observation
		if err := rows.Scan(&obs.Content); err != nil {
			return nil, fmt.Errorf("failed to scan observation: %w", err)
		}

		if p.symbolSearcher != nil {
			// Optionally fetch structural data from Scouter's store based on the observation content
			syms, _ := p.symbolSearcher(ctx, obs.Content)
			if len(syms) > 0 {
				obs.ASTContext = fmt.Sprintf("Correlated Symbol: %s (%s) in %s", syms[0].Name, syms[0].Type, syms[0].Path)
				for i := 0; i < len(syms) && i < 3; i++ {
					obs.StructuralLinks = append(obs.StructuralLinks, syms[i].Path)
				}
			}
		}

		observations = append(observations, obs)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error: %w", err)
	}

	return observations, nil
}

func (p *SQLiteMemoryProvider) SaveObservation(ctx context.Context, project string, mem memory.DistilledMemory) error {
	db, err := p.openDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Use BEGIN IMMEDIATE for write transactions to prevent "database is locked" errors in concurrent environments
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO observations (project, content, created_at)
		VALUES (?, ?, datetime('now'))
	`

	content := fmt.Sprintf("[%s] %s: %s", mem.Type, mem.Title, mem.Content)

	_, err = tx.ExecContext(ctx, query, project, content)
	if err != nil {
		return fmt.Errorf("failed to insert observation: %w", err)
	}

	return tx.Commit()
}

func (p *SQLiteMemoryProvider) SaveSummary(ctx context.Context, project string, summary memory.Summary) error {
	db, err := p.openDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	markdown := p.formatSummaryMarkdown(summary)
	query := `
		INSERT INTO observations (project, content, created_at)
		VALUES (?, ?, datetime('now'))
	`

	_, err = tx.ExecContext(ctx, query, project, markdown)
	if err != nil {
		return fmt.Errorf("failed to insert summary observation: %w", err)
	}

	return tx.Commit()
}

func (p *SQLiteMemoryProvider) formatSummaryMarkdown(s memory.Summary) string {
	var sb strings.Builder
	sb.WriteString("# Engram Distillation Summary\n\n")

	sb.WriteString("## Architectural Decisions\n")
	if len(s.ADRs) == 0 {
		sb.WriteString("- No significant architectural decisions detected.\n")
	} else {
		for _, adr := range s.ADRs {
			sb.WriteString("- ")
			sb.WriteString(adr)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## Root Cause Bug Fixes\n")
	if len(s.BugFixes) == 0 {
		sb.WriteString("- No root cause bug fixes identified.\n")
	} else {
		for _, bf := range s.BugFixes {
			sb.WriteString("- ")
			sb.WriteString(bf)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## Established Patterns\n")
	if len(s.Patterns) == 0 {
		sb.WriteString("- No new patterns or conventions found.\n")
	} else {
		for _, p := range s.Patterns {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (p *SQLiteMemoryProvider) SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error) {
	project := utils.GetRepoName(ctx)

	db, err := p.openDB(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to open database (ro): %w", err)
	}
	defer db.Close()

	sqlQuery := `
		SELECT id, content
		FROM observations
		WHERE project = ? AND content LIKE ? ESCAPE '\'
		ORDER BY created_at DESC
		LIMIT ?
	`
	escapedQuery := strings.ReplaceAll(query, `\`, `\\`)
	escapedQuery = strings.ReplaceAll(escapedQuery, `%`, `\%`)
	escapedQuery = strings.ReplaceAll(escapedQuery, `_`, `\_`)
	likeQuery := "%" + escapedQuery + "%"
	rows, err := db.QueryContext(ctx, sqlQuery, project, likeQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var insights []types.MemoryInsight
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}

		insight := parseSQLiteContent(id, content)
		insights = append(insights, insight)
	}

	return insights, nil
}

func parseSQLiteContent(id, input string) types.MemoryInsight {
	// Replicating logic from internal/store/store.go
	headerRegex := regexp.MustCompile(`^#\s+(.+)$`)
	altHeaderRegex := regexp.MustCompile(`^\[\d+\] #(\d+) \((\w+)\) (?:—|-) (.+)$`)
	whyRegex := regexp.MustCompile(`(?i)\*\*Why\*\*: (.+)$`)
	learnedRegex := regexp.MustCompile(`(?i)\*\*Learned\*\*: (.+)$`)

	insight := types.MemoryInsight{ID: id}
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := headerRegex.FindStringSubmatch(trimmed); matches != nil {
			insight.Title = matches[1]
		} else if matches := altHeaderRegex.FindStringSubmatch(trimmed); matches != nil {
			insight.Type = matches[2]
			insight.Title = matches[3]
		} else if matches := whyRegex.FindStringSubmatch(trimmed); matches != nil {
			insight.Why = matches[1]
		} else if matches := learnedRegex.FindStringSubmatch(trimmed); matches != nil {
			insight.Learned = matches[1]
		}
	}

	return insight
}