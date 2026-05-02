package engine

import (
	"context"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestResolveInterfaces(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_interfaces.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	analyzer := NewAnalysisEngine(s)

	// 1. Setup Interface and Implementation
	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "iface.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "impl.go", Project: "p"})

	// Interface: Shape with method Area() float64
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Shape", Type: "interface", Path: "iface.go"})
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Shape:Area", Type: "method_spec", Signature: "() (float64)", Path: "iface.go"})

	// Implementation: Circle with method Area() float64
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Circle", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Circle.Area", Type: "method", Signature: "() (float64)", Path: "impl.go"})

	// Another struct that DOES NOT match (different signature)
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Square", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "Square.Area", Type: "method", Signature: "(int) (float64)", Path: "impl.go"})

	// 2. Resolve
	if err := analyzer.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("ResolveInterfaces failed: %v", err)
	}

	// 3. Verify
	callers, err := s.GetCallers(ctx, "Shape")
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	found := false
	for _, c := range callers {
		if c.CallerName == "Circle" && c.LinkType == "implements" {
			found = true
		}
		if c.CallerName == "Square" {
			t.Errorf("Square should NOT implement Shape (signature mismatch)")
		}
	}

	if !found {
		t.Errorf("Circle should implement Shape")
	}
}

func TestResolveCentrality(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_centrality.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	analyzer := NewAnalysisEngine(s)

	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "a.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "b.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &store.FileIndex{Path: "c.go", Project: "p"})

	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "A", Type: "func", Path: "a.go"})
	_ = s.SaveSymbol(ctx, &store.Symbol{Name: "B", Type: "func", Path: "b.go"})

	_ = s.SaveCall(ctx, store.Call{CallerName: "A", CalleeName: "B", Path: "a.go", CalleePath: "b.go"})
	_ = s.SaveCall(ctx, store.Call{CallerName: "C", CalleeName: "B", Path: "c.go", CalleePath: "b.go"})

	if err := analyzer.ResolveCentrality(ctx); err != nil {
		t.Fatalf("ResolveCentrality failed: %v", err)
	}

	syms, _ := s.SearchSymbols(ctx, "B", "")
	if len(syms) == 0 {
		t.Fatalf("Symbol B not found")
	}

	if syms[0].Relevance != 2 {
		t.Errorf("expected centrality 2, got %v", syms[0].Relevance)
	}
}
