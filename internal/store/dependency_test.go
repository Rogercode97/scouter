package store

import (
	"context"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestStoreDependencies(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_scouter_deps.db"
	defer os.Remove(dbPath)

	s, err := NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Save dummy dependencies
	deps := []types.Dependency{
		{Name: "github.com/google/uuid", Version: "v1.3.0", Type: "golang", Project: "/path/to/go.mod", Direct: true},
		{Name: "lodash", Version: "4.17.21", Type: "npm", Project: "/path/to/package.json", Direct: true},
		{Name: "golang.org/x/mod", Version: "v0.12.0", Type: "golang", Project: "/path/to/go.mod", Direct: false},
	}

	for _, d := range deps {
		if err := s.SaveDependency(ctx, &d); err != nil {
			t.Fatalf("Failed to save dependency %s: %v", d.Name, err)
		}
	}

	// 2. Test GetDependencies
	results, err := s.GetDependencies(ctx)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(results))
	}

	// 3. Test ClearDependencies
	if err := s.ClearDependencies(ctx); err != nil {
		t.Fatalf("ClearDependencies failed: %v", err)
	}

	results, _ = s.GetDependencies(ctx)
	if len(results) != 0 {
		t.Errorf("Expected 0 dependencies after ClearDependencies, got %d", len(results))
	}
}
