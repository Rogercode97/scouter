package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

// TestDiagnose_ReadOnlyNoMutations pins the doctor contract: a diagnosis
// performs no writes. The target file keeps its exact bytes, the memory
// provider receives zero observations, and no Ledger journal appears.
func TestDiagnose_ReadOnlyNoMutations(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	mockMem := &testMemoryProvider{}
	analyzer := NewAnalysisEngine(s)
	impact := NewImpactEngine(s, nil, mockMem)
	search := NewSearchEngine(s, nil, nil)
	diagnostic := NewDiagnosticEngine(s, analyzer, impact, lsp.NewManager(), search)

	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	content := "package target\n\nfunc broken() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	errorLog := "--- FAIL: TestBroken (0.00s)\n    " + target + ":3: boom\n"

	report, err := diagnostic.Diagnose(ctx, errorLog)
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if report.FailingFile != target {
		t.Errorf("FailingFile = %q, want %q", report.FailingFile, target)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(after) != content {
		t.Errorf("diagnosis mutated the target file")
	}

	if len(mockMem.savedObservations) != 0 {
		t.Errorf("diagnosis wrote %d memory observations, want 0", len(mockMem.savedObservations))
	}

	if _, err := os.Stat(filepath.Join(dir, ".scouter", "staging", "ledger.json")); !os.IsNotExist(err) {
		t.Errorf("diagnosis created a Ledger journal, want none")
	}
}
