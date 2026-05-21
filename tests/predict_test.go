package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
)

func TestIntegration_PredictiveTesting(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_integration_predict.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Prepare Fixture Paths
	sourcePath, _ := filepath.Abs("fixtures/predict_source.go")
	testPath, _ := filepath.Abs("fixtures/predict_source_test.go")
	
	// 2. Index the files
	files := []string{sourcePath, testPath}
	for _, path := range files {
		pointers, calls, err := engine.ParseFile(ctx, path, nil)
		if err != nil {
			t.Fatalf("ParseFile failed for %s: %v", path, err)
		}

		err = s.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
			tx.SaveFileIndex(ctx, &store.FileIndex{
				Path: path,
				Hash: "hash-" + filepath.Base(path),
			})
			for _, p := range pointers {
				t.Logf("Indexing symbol: %s (%s) at line %d in %s", p.Name, p.Type, p.StartLine, path)
				err := tx.SaveSymbol(ctx, &store.Symbol{
					Name:        p.Name,
					Type:        p.Type,
					PackagePath: p.PackagePath,
					Signature:   p.Signature,
					Path:        path,
					StartByte:   p.Range.Start,
					EndByte:     p.Range.End,
					StartLine:   p.StartLine,
					EndLine:     p.EndLine,
				})
				if err != nil {
					return err
				}
			}
			for _, c := range calls {
				t.Logf("Saving call: %s -> %s (callee_path: %q) in %s", c.CallerName, c.CalleeName, c.CalleePath, path)
				err := tx.SaveCall(ctx, store.Call{
					CallerName: c.CallerName,
					CalleeName: c.CalleeName,
					CalleePath: c.CalleePath,
					LinkType:   c.LinkType,
					Path:       path,
					Line:       c.Line,
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Transaction failed for %s: %v", path, err)
		}
	}

	// 3. Mock a git diff that modifies 'Sum' in 'predict_source.go'
	// Sum is at line 3 in predict_source.go
	diff := `--- a/fixtures/predict_source.go
+++ b/fixtures/predict_source.go
@@ -3,1 +3,1 @@
-func Sum(a, b int) int {
+func Sum(a, b, c int) int {`

	// 4. Call engine.PredictTests
	impactEngine := engine.NewImpactEngine(s, nil, nil)
	tests, err := impactEngine.PredictTests(ctx, diff)
	if err != nil {
		t.Fatalf("PredictTests failed: %v", err)
	}

	// 5. Verify results
	if len(tests) == 0 {
		t.Fatalf("expected at least 1 affected test, got 0")
	}

	found := false
	for _, tt := range tests {
		if tt.Name == "TestSum" && tt.File == testPath {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("TestSum not found in affected tests: %+v", tests)
	}
}
