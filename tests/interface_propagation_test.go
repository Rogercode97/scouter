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

func TestInterfacePropagation_Reproduction(t *testing.T) {
	ctx := context.Background()
	dbPath := "repro_interface.db"
	defer os.Remove(dbPath)

	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// 1. Create a sample file with an interface and implementation
	content := `package repro

type Greeter interface {
	Greet(name string) string
}

type EnglishGreeter struct{}

func (g *EnglishGreeter) Greet(name string) string {
	return "Hello, " + name
}
`
	cwd, _ := os.Getwd()
	filePath := filepath.Join(cwd, "repro_interface.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write repro file: %v", err)
	}
	defer os.Remove(filePath)

	// 2. Index and Resolve
	analyzer := engine.NewAnalysisEngine(st)
	// We need to simulate the indexing process manually here since we are in a test
	// and we want to avoid side effects on the main scouter index.
	// TruthEngine.Index uses StreamSymbols internally.
	
	// Actually, let's use TruthEngine to index it properly
	te := engine.NewTruthEngine(st, analyzer, nil, nil, nil, nil, nil, nil, nil)
	if err := te.Index(ctx, filePath); err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// 3. Verify 'implements' link exists
	calls, err := st.GetCallers(ctx, "Greeter", 0, 0)
	if err != nil {
		t.Fatalf("failed to get callers: %v", err)
	}

	found := false
	for _, c := range calls {
		if c.CallerName == "EnglishGreeter" && c.LinkType == "implements" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected 'implements' link from EnglishGreeter to Greeter, but not found")
	}

	// 4. Try to propagate rename of Greeter:Greet
	// Current RippleEngine uses BFSPropagationStrategy which only follows callers.
	// It DOES NOT follow implementations.
	
	lspMgr := lsp.NewManager()
	defer lspMgr.Close()
	impact := engine.NewImpactEngine(st, lspMgr)
	strategy := engine.NewBFSPropagationStrategy(st, impact)
	
	// We want to see if renaming Greeter:Greet (the method) finds EnglishGreeter.Greet
	tasks := strategy.Discover(ctx, "Greeter:Greet", 2)
	
	discovered := make(map[string]bool)
	for task, err := range tasks {
		if err != nil {
			t.Fatalf("discovery failed: %v", err)
		}
		discovered[task.SymbolName] = true
	}

	// EXPECTED FAILURE: EnglishGreeter.Greet should be discovered but currently it isn't
	if !discovered["EnglishGreeter.Greet"] {
		t.Logf("CONFIRMED: EnglishGreeter.Greet was NOT discovered during interface method rename propagation.")
	} else {
		t.Errorf("SURPRISE: EnglishGreeter.Greet WAS discovered. Maybe the feature already exists?")
	}
}
