package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func TestInterfacePropagation_Omniscience(t *testing.T) {
	ctx := context.Background()
	dbPath := "omniscience.db"
	defer os.Remove(dbPath)

	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// 1. Create a sample file with an interface and two implementations
	content := `package repro

type Greeter interface {
	Greet(name string) string
}

type EnglishGreeter struct{}

func (g *EnglishGreeter) Greet(name string) string {
	return "Hello, " + name
}

type SpanishGreeter struct{}

func (g *SpanishGreeter) Greet(name string) string {
	return "Hola, " + name
}

func Welcome(g Greeter) {
	g.Greet("Scouter")
}
`
	cwd, _ := os.Getwd()
	filePath := filepath.Join(cwd, "repro_omniscience.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write repro file: %v", err)
	}
	defer os.Remove(filePath)

	// 2. Index and Resolve
	analyzer := engine.NewAnalysisEngine(st)
	te := engine.NewTruthEngine(st, analyzer, nil, nil, nil, nil, nil, nil, nil)
	if err := te.Index(ctx, filePath); err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	lspMgr := lsp.NewManager()
	defer lspMgr.Close()
	impact := engine.NewImpactEngine(st, lspMgr)
	strategy := engine.NewBFSPropagationStrategy(st, impact)

	t.Run("Downward: Interface to Implementations", func(t *testing.T) {
		tasks := strategy.Discover(ctx, "Greeter:Greet", 2)
		discovered := make(map[string]bool)
		for task, err := range tasks {
			if err != nil {
				t.Fatalf("discovery failed: %v", err)
			}
			discovered[task.SymbolName] = true
		}

		if !discovered["EnglishGreeter.Greet"] {
			t.Errorf("expected EnglishGreeter.Greet to be discovered")
		}
		if !discovered["SpanishGreeter.Greet"] {
			t.Errorf("expected SpanishGreeter.Greet to be discovered")
		}
		if !discovered["Welcome"] {
			t.Errorf("expected Welcome (caller) to be discovered")
		}
	})

	t.Run("Upward and Siblings: Implementation to Interface and Others", func(t *testing.T) {
		// Starting from EnglishGreeter.Greet
		tasks := strategy.Discover(ctx, "EnglishGreeter.Greet", 3)
		discovered := make(map[string]bool)
		for task, err := range tasks {
			if err != nil {
				t.Fatalf("discovery failed: %v", err)
			}
			discovered[task.SymbolName] = true
		}

		if !discovered["Greeter:Greet"] {
			t.Errorf("expected Greeter:Greet (interface) to be discovered upward")
		}
		if !discovered["SpanishGreeter.Greet"] {
			t.Errorf("expected SpanishGreeter.Greet (sibling) to be discovered via interface")
		}
		if !discovered["Welcome"] {
			t.Errorf("expected Welcome (caller of interface) to be discovered via interface")
		}
	})
}
