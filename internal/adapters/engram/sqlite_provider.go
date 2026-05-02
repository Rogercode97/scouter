package engram

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	_ "modernc.org/sqlite"
)

/**
 * ⚔️ HAKAISHIN SQLITE ADAPTER (WAVE 7)
 */
type SQLiteMemoryProvider struct {
	dbPath       string
	scouterStore store.Repository
}

func NewSQLiteMemoryProvider(dbPath string) *SQLiteMemoryProvider {
	return &SQLiteMemoryProvider{
		dbPath: dbPath,
	}
}

func (p *SQLiteMemoryProvider) WithStore(scouterStore store.Repository) *SQLiteMemoryProvider {
	p.scouterStore = scouterStore
	return p
}

func (p *SQLiteMemoryProvider) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
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

		if p.scouterStore != nil {
			// Optionally fetch structural data from Scouter's store based on the observation content
			syms, _ := p.scouterStore.SearchSymbols(ctx, obs.Content, "", 0, 0)
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
