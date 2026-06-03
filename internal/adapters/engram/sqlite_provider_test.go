package engram

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

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
