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

func TestHierarchicalInterface_Propagation(t *testing.T) {
	ctx := context.Background()
	dbPath := "hierarchical.db"
	defer os.Remove(dbPath)

	st, err := store.NewStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// GIVEN Interface A with M1()
	// AND Interface B embeds A
	// AND Struct C implements B
	content := `package tests

type A interface {
	M1()
}

type B interface {
	A
	M2()
}

type C struct{}

func (c *C) M1() {}
func (c *C) M2() {}

func UseA(a A) {
	a.M1()
}

func UseB(b B) {
	b.M1()
	b.M2()
}
`
	cwd, _ := os.Getwd()
	filePath := filepath.Join(cwd, "repro_hierarchy.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write repro file: %v", err)
	}
	defer os.Remove(filePath)

	// Index and Resolve
	analyzer := engine.NewAnalysisEngine(st)
	te := engine.NewTruthEngine(st, engine.WithAnalyzer(analyzer))
	if err := te.Index(ctx, filePath); err != nil {
		t.Fatalf("failed to index: %v", err)
	}
	
	var pkgPath string
	for sym, err := range st.GetAllSymbols(ctx) {
		if err == nil && sym.Name == "A" {
			pkgPath = sym.PackagePath
			break
		}
	}
	
	if pkgPath == "" {
		t.Fatal("could not find pkgPath for A")
	}

	if err := analyzer.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	lspMgr := lsp.GetGlobalManager()
	impact := engine.NewImpactEngine(st, lspMgr, nil)
	strategy := engine.NewBFSPropagationStrategy(st, impact)

	fq := func(name string) string { return pkgPath + "." + name }

	t.Run("Propagation from A.M1 to C.M1 via B", func(t *testing.T) {
		// Debug: Print all symbols
		t.Log("Symbols in store:")
		for sym, err := range st.GetAllSymbols(ctx) {
			if err == nil {
				t.Logf("  Name: %s, Type: %s, Pkg: %s", sym.Name, sym.Type, sym.PackagePath)
			}
		}

		// Debug: Print all calls in the store
		t.Log("Calls in store:")
		for call, err := range st.GetAllCalls(ctx) {
			if err == nil {
				t.Logf("  Caller: %s, Callee: %s, Type: %s", call.CallerName, call.CalleeName, call.LinkType)
			}
		}

		tasks := strategy.Discover(ctx, fq("A") + ".M1", 3)
		discovered := make(map[string]bool)
		for task, err := range tasks {
			if err != nil {
				t.Fatalf("discovery failed: %v", err)
			}
			discovered[task.ImpactedSymbol] = true
		}

		if !discovered[fq("UseA")] {
			t.Errorf("expected UseA to be discovered")
		}
		
		// This is what we are implementing:
		if !discovered[fq("UseB")] {
			t.Errorf("expected UseB to be discovered (via B embedding A)")
		}
		
		if !discovered[fq("C") + ".M1"] {
			t.Errorf("expected C.M1 to be discovered")
		}
	})
}
