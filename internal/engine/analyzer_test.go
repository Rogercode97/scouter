package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestResolveInterfaces(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scouter-analyzer-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module testanalyzer\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	content := `
package testanalyzer

type Shape interface {
	Area() float64
}

type Circle struct{}
func (c Circle) Area() float64 { return 0 }

type Square struct{}
func (s *Square) Area() float64 { return 0 }

type NotAShape struct{}
func (n NotAShape) Area(x int) float64 { return 0 }
`
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Parse and save symbols
	pointers, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if err := s.SaveFileIndex(ctx, &store.FileIndex{Path: filePath, Project: "test"}); err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	for _, p := range pointers {
		_ = s.SaveSymbol(ctx, &store.Symbol{
			Name:         p.Name,
			Type:         p.Type,
			PackagePath:  p.PackagePath,
			ReceiverType: p.ReceiverType,
			Path:         filePath,
			Signature:    p.Signature,
		})
	}

	analyzer := NewAnalysisEngine(s)
	analyzer.ProjectRoot = tmpDir

	if err := analyzer.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("ResolveInterfaces failed: %v", err)
	}

	// Verify implements
	callers, err := s.GetCallers(ctx, "testanalyzer.Shape", 0, 0)
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	foundCircle := false
	foundSquare := false
	for _, c := range callers {
		if c.CallerName == "testanalyzer.Circle" && c.LinkType == "implements" {
			foundCircle = true
		}
		if c.CallerName == "testanalyzer.Square" && c.LinkType == "implements" {
			foundSquare = true
		}
		if strings.Contains(c.CallerName, "NotAShape") {
			t.Errorf("NotAShape should NOT implement Shape")
		}
	}

	if !foundCircle {
		t.Errorf("Circle should implement Shape")
	}
	if !foundSquare {
		t.Errorf("Square should implement Shape")
	}

	// Verify satisfies (methods)
	satisfiers, err := s.GetCallers(ctx, "testanalyzer.Shape.Area", 0, 0)
	if err != nil {
		t.Fatalf("GetCallers for methods failed: %v", err)
	}

	foundCircleArea := false
	for _, c := range satisfiers {
		if c.CallerName == "testanalyzer.Circle.Area" && c.LinkType == "satisfies" {
			foundCircleArea = true
		}
	}
	if !foundCircleArea {
		t.Errorf("Circle.Area should satisfy Shape.Area")
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

	syms, _ := s.SearchSymbols(ctx, "B", "", 0, 0)
	if len(syms) == 0 {
		t.Fatalf("Symbol B not found")
	}

	if syms[0].Relevance != 2 {
		t.Errorf("expected centrality 2, got %v", syms[0].Relevance)
	}
}
