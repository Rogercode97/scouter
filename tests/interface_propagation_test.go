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
	content := `package tests

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
	te := engine.NewTruthEngine(st, engine.WithAnalyzer(analyzer))
	if err := te.Index(ctx, filePath); err != nil {
		t.Fatalf("failed to index: %v", err)
	}
	
	// Get the package path assigned by the parser
	var pkgPath string
	for sym, err := range st.GetAllSymbols(ctx) {
		if err == nil && sym.Name == "Greeter" {
			pkgPath = sym.PackagePath
			break
		}
	}
	
	if pkgPath == "" {
		t.Fatal("could not find pkgPath for Greeter")
	}

	if err := analyzer.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	lspMgr := lsp.GetGlobalManager()
	impact := engine.NewImpactEngine(st, lspMgr, nil)
	strategy := engine.NewBFSPropagationStrategy(st, impact)

	// FQN helpers
	fq := func(name string) string { return pkgPath + "." + name }

	t.Run("Downward: Interface to Implementations", func(t *testing.T) {
		tasks := strategy.Discover(ctx, fq("Greeter") + ".Greet", 2)
		discovered := make(map[string]bool)
		for task, err := range tasks {
			if err != nil {
				t.Fatalf("discovery failed: %v", err)
			}
			discovered[task.ImpactedSymbol] = true
		}

		if !discovered[fq("EnglishGreeter") + ".Greet"] {
			t.Errorf("expected %s to be discovered", fq("EnglishGreeter") + ".Greet")
		}
		if !discovered[fq("SpanishGreeter") + ".Greet"] {
			t.Errorf("expected %s to be discovered", fq("SpanishGreeter") + ".Greet")
		}
		// Welcome calls the interface method
		if !discovered[fq("Welcome")] {
			t.Errorf("expected %s (caller) to be discovered", fq("Welcome"))
		}
	})

	t.Run("Upward and Siblings: Implementation to Interface and Others", func(t *testing.T) {
		tasks := strategy.Discover(ctx, fq("EnglishGreeter") + ".Greet", 3)
		discovered := make(map[string]bool)
		for task, err := range tasks {
			if err != nil {
				t.Fatalf("discovery failed: %v", err)
			}
			discovered[task.ImpactedSymbol] = true
		}

		if !discovered[fq("Greeter") + ".Greet"] {
			t.Errorf("expected %s (interface) to be discovered upward", fq("Greeter") + ".Greet")
		}
		if !discovered[fq("SpanishGreeter") + ".Greet"] {
			t.Errorf("expected %s (sibling) to be discovered via interface", fq("SpanishGreeter") + ".Greet")
		}
		if !discovered[fq("Welcome")] {
			t.Errorf("expected %s (caller of interface) to be discovered via interface", fq("Welcome"))
		}
	})
}
