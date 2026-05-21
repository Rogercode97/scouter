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
	_ "modernc.org/sqlite"
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

func (p *SQLiteMemoryProvider) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	// Open in read-only mode for safety
	dsn := fmt.Sprintf("file:%s?mode=ro", p.dbPath)
	db, err := sql.Open("sqlite", dsn)
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
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	query := `
		INSERT INTO observations (project, content, created_at)
		VALUES (?, ?, datetime('now'))
	`

	content := fmt.Sprintf("[%s] %s: %s", mem.Type, mem.Title, mem.Content)

	_, err = db.ExecContext(ctx, query, project, content)
	if err != nil {
		return fmt.Errorf("failed to insert observation: %w", err)
	}

	return nil
}

func (p *SQLiteMemoryProvider) SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error) {
	project := utils.GetRepoName(ctx)

	// Open in read-only mode
	dsn := fmt.Sprintf("file:%s?mode=ro", p.dbPath)
	db, err := sql.Open("sqlite", dsn)
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