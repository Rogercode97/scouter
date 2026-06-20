package mcp

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/store"
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)

// MockDistiller for integration testing
type MockDistiller struct {
	Memories []memory.DistilledMemory
}

func (m *MockDistiller) Distill(ctx context.Context, logs []memory.Observation) (memory.Summary, error) {
	return memory.Summary{}, nil
}

func (m *MockDistiller) DistillTranscript(ctx context.Context, transcript []memory.Message) ([]memory.DistilledMemory, error) {
	return m.Memories, nil
}

func TestDream_PassiveIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Setup Mock Engram DB (Real File for consistency)
	tmpDB, err := os.MkdirTemp("", "engram-db-*")
	if err != nil {
		t.Fatalf("failed to create tmp db dir: %v", err)
	}
	defer os.RemoveAll(tmpDB)
	dbPath := filepath.Join(tmpDB, "engram.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT,
		content TEXT,
		created_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	db.Close()

	// 2. Setup Scouter Server with Mocked Services
	st, _ := store.NewStore(ctx, ":memory:")
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server := setupMockServer(st, logger)

	// Inject Test-friendly AppService
	memoryProvider := engram.NewSQLiteMemoryProvider(dbPath)
	mockDistiller := &MockDistiller{
		Memories: []memory.DistilledMemory{
			{Type: "architecture", Title: "Test Decision", Content: "We decided to use SQLite for tests."},
		},
	}
	server.appService = memory.NewAppService(memoryProvider)

	// 3. Manually trigger PassiveDistill directly
	// This verifies the integration between AppService and SQLiteMemoryProvider

	// PassiveDistill in AppService is what actually saves to the DB.
	// postToolHook sets the distiller on appService using req.Session.
	// Since we already injected a MockDistiller into appService, we can call it directly.

	err = server.appService.PassiveDistill(ctx, "scouter", []memory.Message{
		{Role: "user", Content: "Let's use SQLite for memories."},
		{Role: "assistant", Content: "Good idea. I'll implement it."},
	}, mockDistiller)
	if err != nil {
		t.Fatalf("PassiveDistill failed: %v", err)
	}

	// 4. Verify observation was saved
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite for verification: %v", err)
	}
	defer db.Close()

	var content string
	err = db.QueryRow("SELECT content FROM observations LIMIT 1").Scan(&content)
	if err != nil {
		t.Fatalf("failed to find observation in DB: %v", err)
	}

	expectedPrefix := "[architecture]"
	if content[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected content to start with %s, got %s", expectedPrefix, content)
	}

	expectedTitle := "Test Decision"
	// Content format is "[type] title: content"
	if content[len(expectedPrefix)+1:len(expectedPrefix)+1+len(expectedTitle)] != expectedTitle {
		t.Errorf("expected content to contain title %s, got %s", expectedTitle, content)
	}
}
