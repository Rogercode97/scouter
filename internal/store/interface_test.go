package store

import (
	"os"
	"testing"
)

func TestResolveInterfaces(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_interfaces.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Setup Interface and Implementation
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "iface.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "impl.go", Project: "p"})

	// Interface: Shape with method Area() float64
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape", Type: "interface", Path: "iface.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape:Area", Type: "method_spec", Signature: "() (float64)", Path: "iface.go"})

	// Implementation: Circle with method Area() float64
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle.Area", Type: "method", Signature: "() (float64)", Path: "impl.go"})

	// Another struct that DOES NOT match (different signature)
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Square", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Square.Area", Type: "method", Signature: "(int) (float64)", Path: "impl.go"})

	// 2. Resolve
	if err := s.ResolveInterfaces(ctx); err != nil {
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
		t.Error("Expected Circle to implement Shape")
	}
}
