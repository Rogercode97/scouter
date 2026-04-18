package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	trackCount   int
	trackCountMu sync.Mutex
)

// Tracker manages token savings tracking in SQLite.
type Tracker struct {
	db      *sql.DB
	dbPath  string
	once    sync.Once
	wg      sync.WaitGroup
	initErr error
}

// NewTracker opens or creates a SQLite database for tracking (immediate open).
func NewTracker(ctx context.Context, dbPath string) (*Tracker, error) {
	t := &Tracker{dbPath: dbPath}
	if err := t.ensureOpen(ctx); err != nil {
		return nil, err
	}
	return t, nil
}

// NewLazyTracker creates a tracker that defers DB opening until first use.
func NewLazyTracker(dbPath string) *Tracker {
	return &Tracker{dbPath: dbPath}
}

// WarmUp starts opening the DB in the background.
// Call this before command execution so SQLite init overlaps with the command.
func (t *Tracker) WarmUp(ctx context.Context) {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		_ = t.ensureOpen(ctx)
	}()
}

// Wait waits for any background initialization to complete.
func (t *Tracker) Wait() {
	t.wg.Wait()
}

func (t *Tracker) ensureOpen(ctx context.Context) error {
	t.once.Do(func() {
		dir := filepath.Dir(t.dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.initErr = fmt.Errorf("create db dir: %w", err)
			return
		}

		db, err := sql.Open("sqlite", t.dbPath)
		if err != nil {
			t.initErr = fmt.Errorf("open db: %w", err)
			return
		}

		if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
			// Check if we need to migrate column name snip_cmd -> scouter_cmd
			if _, renameErr := db.ExecContext(ctx, "ALTER TABLE commands RENAME COLUMN snip_cmd TO scouter_cmd"); renameErr == nil {
				// Successfully renamed
			} else {
				_ = db.Close()
				t.initErr = fmt.Errorf("create table: %w", err)
				return
			}
		}

		t.db = db
	})
	return t.initErr
}

// Track records a filtered command execution.
func (t *Tracker) Track(ctx context.Context, originalCmd, scouterCmd string, inputTokens, outputTokens int, execTimeMs int64) error {
	if err := t.ensureOpen(ctx); err != nil {
		return fmt.Errorf("track: %w", err)
	}

	saved := inputTokens - outputTokens
	pct := 0.0
	if inputTokens > 0 {
		pct = float64(saved) / float64(inputTokens) * 100
	}

	if _, err := t.db.ExecContext(ctx, insertSQL, originalCmd, scouterCmd, inputTokens, outputTokens, saved, pct, execTimeMs); err != nil {
		return fmt.Errorf("track: %w", err)
	}

	// Periodic Cleanup via Sampling (Ghost 2: Optimization)
	trackCountMu.Lock()
	trackCount++
	shouldCleanup := trackCount%10 == 0
	trackCountMu.Unlock()

	if shouldCleanup {
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = t.db.ExecContext(cleanupCtx, cleanupSQL)
		}()
	}

	return nil
}

// TrackPassthrough records a passthrough (unfiltered) command.
func (t *Tracker) TrackPassthrough(ctx context.Context, cmd string, tokens int, execTimeMs int64) error {
	return t.Track(ctx, cmd, cmd, tokens, tokens, execTimeMs)
}

// GetSummary returns aggregate tracking stats.
func (t *Tracker) GetSummary(ctx context.Context) (*Summary, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	var s Summary
	err := t.db.QueryRowContext(ctx, summarySQL).Scan(&s.TotalCommands, &s.TotalSaved, &s.AvgSavings, &s.TotalTimeMs)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	return &s, nil
}

// GetDaily returns daily stats for the last N days.
func (t *Tracker) GetDaily(ctx context.Context, days int) ([]DayStats, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("daily: %w", err)
	}
	if days <= 0 {
		days = 7
	}
	rows, err := t.db.QueryContext(ctx, dailySQL, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("daily: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []DayStats
	for rows.Next() {
		var d DayStats
		if err := rows.Scan(&d.Day, &d.Commands, &d.InputTokens, &d.OutputTokens, &d.SavedTokens, &d.AvgSavings); err != nil {
			return nil, fmt.Errorf("daily scan: %w", err)
		}
		stats = append(stats, d)
	}
	return stats, rows.Err()
}

// GetRecent returns the last N tracked commands.
func (t *Tracker) GetRecent(ctx context.Context, n int) ([]CommandRecord, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("recent: %w", err)
	}
	rows, err := t.db.QueryContext(ctx, recentSQL, n)
	if err != nil {
		return nil, fmt.Errorf("recent: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []CommandRecord
	for rows.Next() {
		var r CommandRecord
		if err := rows.Scan(&r.OriginalCmd, &r.ScouterCmd, &r.InputTokens, &r.OutputTokens, &r.SavedTokens, &r.SavingsPct, &r.ExecTimeMs, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("recent scan: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// GetByCommand returns top N commands by tokens saved.
func (t *Tracker) GetByCommand(ctx context.Context, limit int) ([]CommandStats, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("by command: %w", err)
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := t.db.QueryContext(ctx, byCommandSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("by command: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []CommandStats
	for rows.Next() {
		var s CommandStats
		if err := rows.Scan(&s.Command, &s.Count, &s.InputTokens, &s.OutputTokens, &s.SavedTokens, &s.AvgSavings); err != nil {
			return nil, fmt.Errorf("by command scan: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetWeekly returns weekly stats for the last N weeks.
func (t *Tracker) GetWeekly(ctx context.Context, weeks int) ([]PeriodStats, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("weekly: %w", err)
	}
	if weeks <= 0 {
		weeks = 4
	}
	days := weeks * 7
	rows, err := t.db.QueryContext(ctx, weeklySQL, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("weekly: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []PeriodStats
	for rows.Next() {
		var s PeriodStats
		if err := rows.Scan(&s.Period, &s.Commands, &s.InputTokens, &s.OutputTokens, &s.SavedTokens, &s.AvgSavings); err != nil {
			return nil, fmt.Errorf("weekly scan: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetMonthly returns monthly stats for the last N months.
func (t *Tracker) GetMonthly(ctx context.Context, months int) ([]PeriodStats, error) {
	if err := t.ensureOpen(ctx); err != nil {
		return nil, fmt.Errorf("monthly: %w", err)
	}
	if months <= 0 {
		months = 6
	}
	days := months * 30
	rows, err := t.db.QueryContext(ctx, monthlySQL, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("monthly: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []PeriodStats
	for rows.Next() {
		var s PeriodStats
		if err := rows.Scan(&s.Period, &s.Commands, &s.InputTokens, &s.OutputTokens, &s.SavedTokens, &s.AvgSavings); err != nil {
			return nil, fmt.Errorf("monthly scan: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// Close closes the database connection.
func (t *Tracker) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

// DBPath resolves the tracking database path.
func DBPath(configPath string) string {
	if p := os.Getenv("SCOUTER_DB_PATH"); p != "" {
		return p
	}
	if configPath != "" {
		return configPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "scouter", "scouter.db")
}
