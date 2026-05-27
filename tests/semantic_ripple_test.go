package tests

import (
	"context"
	"testing"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
)

func TestSemanticRipple_CrossPackage(t *testing.T) {
	ctx := context.Background()
	
	cwd, _ := os.Getwd()
	t.Logf("CWD: %s", cwd)
	
	var fixtureDir string
	if _, err := os.Stat("fixtures/semantic_ripple"); err == nil {
		fixtureDir, _ = filepath.Abs("fixtures/semantic_ripple")
	} else if _, err := os.Stat("tests/fixtures/semantic_ripple"); err == nil {
		fixtureDir, _ = filepath.Abs("tests/fixtures/semantic_ripple")
	} else {
		t.Fatalf("Could not find fixtures/semantic_ripple")
	}

	t.Logf("Fixture Dir: %s", fixtureDir)
	
	// Check files
	var files []string
	filepath.Walk(fixtureDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	t.Logf("Found %d go files in fixture", len(files))

	os.MkdirAll(filepath.Join(fixtureDir, ".scouter"), 0755)
	dbPath := filepath.Join(fixtureDir, ".scouter", "scouter_test.db")
	os.Remove(dbPath)
	
	db, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	analyzer := engine.NewAnalysisEngine(db)
	analyzer.ProjectRoot = fixtureDir
	
	lspMgr := lsp.NewManager()
	defer lspMgr.Close()
	
	te := engine.NewTruthEngine(db, engine.WithAnalyzer(analyzer), engine.WithLSP(lspMgr))
	
	if err := te.Index(ctx, fixtureDir); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	count := 0
	for sym, err := range db.GetAllSymbols(ctx) {
		if err != nil { t.Fatal(err) }
		t.Logf("Symbol: %s (%s) pkg=%s", sym.Name, sym.Type, sym.PackagePath)
		count++
	}
	t.Logf("Total symbols in DB: %d", count)

	universe, _ := analyzer.BuildTypeUniverse()
	t.Logf("Packages in Universe: %d", len(universe))
	for p := range universe { t.Logf("  - %s", p) }

	if err := analyzer.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("ResolveInterfaces failed: %v", err)
	}

	foundImplements := false
	for call, err := range db.GetAllCalls(ctx) {
		if err != nil { t.Fatal(err) }
		t.Logf("Call: %s -> %s (%s)", call.CallerName, call.CalleeName, call.LinkType)
		if call.LinkType == "implements" {
			foundImplements = true
		}
	}

	if !foundImplements {
		t.Errorf("Expected 'implements' link not found")
	}
}
