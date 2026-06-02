package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/types"
)

type mockMemoryProvider struct{}

func (m *mockMemoryProvider) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	return nil, nil
}
func (m *mockMemoryProvider) SaveObservation(ctx context.Context, project string, mem memory.DistilledMemory) error {
	return nil
}
func (m *mockMemoryProvider) SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error) {
	return nil, nil
}
func (m *mockMemoryProvider) SaveSummary(ctx context.Context, project string, summary memory.Summary) error {
	return nil
}

func BenchmarkMassiveIndexing(b *testing.B) {
	ctx := context.Background()
	
	// Create a temporary directory for the benchmark
	tmpDir, err := os.MkdirTemp("", "scouter-benchmark-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 1000 small Go files to simulate a large project
	for i := 0; i < 1000; i++ {
		fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.go", i))
		content := fmt.Sprintf(`package main
import "fmt"
func Function_%d() {
	fmt.Println("Hello from %d")
}
`, i, i)
		if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Initialize Store with a temp DB
	dbPath := filepath.Join(tmpDir, "benchmark.db")
	s, err := store.NewStore(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	// Initialize TruthEngine
	engine := NewTruthEngine(s, WithMemory(&mockMemoryProvider{}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear data before each run if needed, but for benchmark we usually want to measure the full ingestion
		start := time.Now()
		err := engine.Index(ctx, tmpDir)
		if err != nil {
			b.Fatalf("Index failed: %v", err)
		}
		b.Logf("Indexed 1000 files in %v", time.Since(start))
	}
}
