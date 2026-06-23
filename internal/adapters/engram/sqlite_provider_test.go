package engram

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/types"
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestDiscoverDBPath(t *testing.T) {
	t.Run("returns default path in home dir", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home dir: %v", err)
		}
		expected := filepath.Join(home, ".engram", "observations.db")

		path, err := DiscoverDBPath()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})

	t.Run("honors ENGRAM_DB_PATH environment variable", func(t *testing.T) {
		customPath := "/tmp/custom_engram.db"
		os.Setenv("ENGRAM_DB_PATH", customPath)
		defer os.Unsetenv("ENGRAM_DB_PATH")

		path, err := DiscoverDBPath()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != customPath {
			t.Errorf("expected %s, got %s", customPath, path)
		}
	})
}

func TestSQLiteMemoryProvider_SearchInsights(t *testing.T) {
	// Create a temporary database
	dbPath := filepath.Join(t.TempDir(), "engram.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE observations (
			id INTEGER PRIMARY KEY,
			project TEXT,
			content TEXT,
			created_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test data
	content1 := "# Title 1\n**Why**: Reason 1\n**Learned**: Lesson 1"
	_, err = db.Exec(`
		INSERT INTO observations (id, project, content, created_at)
		VALUES (1, 'scouter', ?, datetime('now'))
	`, content1)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	provider := NewSQLiteMemoryProvider(dbPath)

	t.Run("finds and parses insights", func(t *testing.T) {
		// Mock context with project
		// Note: The original Store.GetMemoryInsights uses utils.GetRepoName(ctx)
		// But SQLiteMemoryProvider should probably take project as arg or use a filter.
		// Task 2 signature: SearchInsights(ctx context.Context, query string, limit int)
		// Wait, if it doesn't take project, how does it filter?
		// Ah, the task says: "Query the observations table for the current project."
		// Maybe it uses utils.GetRepoName(ctx).

		ctx := context.Background()
		insights, err := provider.SearchInsights(ctx, "Title 1", 5)
		if err != nil {
			t.Fatalf("SearchInsights failed: %v", err)
		}

		if len(insights) == 0 {
			t.Fatal("expected at least one insight, got zero")
		}

		if insights[0].ID != "1" {
			t.Errorf("expected ID 1, got %s", insights[0].ID)
		}
		if insights[0].Why != "Reason 1" {
			t.Errorf("expected Why 'Reason 1', got '%s'", insights[0].Why)
		}
		if insights[0].Learned != "Lesson 1" {
			t.Errorf("expected Learned 'Lesson 1', got '%s'", insights[0].Learned)
		}
	})

	t.Run("returns empty if no matches", func(t *testing.T) {
		ctx := context.Background()
		insights, err := provider.SearchInsights(ctx, "NonExistent", 5)
		if err != nil {
			t.Fatalf("SearchInsights failed: %v", err)
		}
		if len(insights) != 0 {
			t.Errorf("expected zero insights, got %d", len(insights))
		}
	})

	t.Run("handles multiple matches and limit", func(t *testing.T) {
		// Insert more data
		_, _ = db.Exec(`
			INSERT INTO observations (id, project, content, created_at)
			VALUES (2, 'scouter', '# Title 2', datetime('now', '+1 minute')),
			       (3, 'scouter', '# Title 3', datetime('now', '+2 minutes'))
		`)

		ctx := context.Background()
		insights, err := provider.SearchInsights(ctx, "Title", 2)
		if err != nil {
			t.Fatalf("SearchInsights failed: %v", err)
		}

		if len(insights) != 2 {
			t.Errorf("expected 2 insights (limit), got %d", len(insights))
		}
		// Ordered by created_at DESC, so Title 3 should be first
		if insights[0].Title != "Title 3" {
			t.Errorf("expected Title 3, got %s", insights[0].Title)
		}
	})
}

func TestSQLiteMemoryProvider_SaveAndGetObservations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "engram_save_get.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE observations (
			id INTEGER PRIMARY KEY,
			project TEXT,
			content TEXT,
			created_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	provider := NewSQLiteMemoryProvider(dbPath)
	ctx := context.Background()

	t.Run("SaveObservation", func(t *testing.T) {
		mem := memory.DistilledMemory{
			Type:    "BugFix",
			Title:   "Fixed nil panic",
			Content: "The pointer was nil",
		}
		err := provider.SaveObservation(ctx, "scouter", mem)
		if err != nil {
			t.Fatalf("SaveObservation failed: %v", err)
		}

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM observations WHERE project = 'scouter'").Scan(&count)
		if err != nil || count != 1 {
			t.Fatalf("expected 1 observation, got %d, err: %v", count, err)
		}
	})

	t.Run("GetRecentObservations", func(t *testing.T) {
		obs, err := provider.GetRecentObservations(ctx, "scouter", 24)
		if err != nil {
			t.Fatalf("GetRecentObservations failed: %v", err)
		}
		if len(obs) != 1 {
			t.Fatalf("expected 1 observation, got %d", len(obs))
		}
		expectedContent := "[BugFix] Fixed nil panic: The pointer was nil"
		if obs[0].Content != expectedContent {
			t.Errorf("expected %q, got %q", expectedContent, obs[0].Content)
		}
	})

	t.Run("GetRecentObservations with symbolSearcher", func(t *testing.T) {
		provider.WithSymbolSearcher(func(ctx context.Context, query string) ([]types.Symbol, error) {
			return []types.Symbol{
				{Name: "Foo", Type: "func", Path: "foo.go"},
			}, nil
		})
		
		obs, err := provider.GetRecentObservations(ctx, "scouter", 24)
		if err != nil {
			t.Fatalf("GetRecentObservations failed: %v", err)
		}
		if len(obs) != 1 {
			t.Fatalf("expected 1 observation, got %d", len(obs))
		}
		if obs[0].ASTContext == "" {
			t.Errorf("expected ASTContext to be populated")
		}
	})
}

func TestSQLiteMemoryProvider_SaveSummary(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "engram_summary.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE observations (
			id INTEGER PRIMARY KEY,
			project TEXT,
			content TEXT,
			created_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	provider := NewSQLiteMemoryProvider(dbPath)
	ctx := context.Background()

	t.Run("SaveSummary formats and saves correctly", func(t *testing.T) {
		summary := memory.Summary{
			ADRs:     []string{"Use SQLite"},
			BugFixes: []string{"Fix race condition"},
			Patterns: []string{"Use context"},
		}
		err := provider.SaveSummary(ctx, "scouter", summary)
		if err != nil {
			t.Fatalf("SaveSummary failed: %v", err)
		}

		var content string
		err = db.QueryRow("SELECT content FROM observations WHERE project = 'scouter'").Scan(&content)
		if err != nil {
			t.Fatalf("failed to get summary content: %v", err)
		}

		if !strings.Contains(content, "Use SQLite") {
			t.Errorf("expected content to contain 'Use SQLite', got %s", content)
		}
		if !strings.Contains(content, "Fix race condition") {
			t.Errorf("expected content to contain 'Fix race condition', got %s", content)
		}
		if !strings.Contains(content, "Use context") {
			t.Errorf("expected content to contain 'Use context', got %s", content)
		}
	})

	t.Run("SaveSummary formats empty slices correctly", func(t *testing.T) {
		summary := memory.Summary{}
		err := provider.SaveSummary(ctx, "empty_proj", summary)
		if err != nil {
			t.Fatalf("SaveSummary failed: %v", err)
		}

		var content string
		err = db.QueryRow("SELECT content FROM observations WHERE project = 'empty_proj'").Scan(&content)
		if err != nil {
			t.Fatalf("failed to get summary content: %v", err)
		}

		if !strings.Contains(content, "No significant architectural decisions detected.") {
			t.Errorf("expected content to contain 'No significant architectural decisions detected.', got %s", content)
		}
	})
}
