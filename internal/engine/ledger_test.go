package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLedger_BaselineCapture(t *testing.T) {
	t.Run("existing file staged with empty original captures content and mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "existing.go")
		initialContent := "package existing\nconst Version = 1\n"
		fileMode := os.FileMode(0755)

		if err := os.WriteFile(testPath, []byte(initialContent), fileMode); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
		if err := os.Chmod(testPath, fileMode); err != nil {
			t.Fatalf("failed to chmod test file: %v", err)
		}

		ledger := NewLedger()
		patch := Patch{
			FilePath:   testPath,
			NewContent: "package existing\nconst Version = 2\n",
		}

		if err := ledger.Stage(testPath, patch); err != nil {
			t.Fatalf("Stage failed: %v", err)
		}

		staged := ledger.GetStaged()
		if len(staged) != 1 {
			t.Fatalf("expected 1 staged patch, got %d", len(staged))
		}

		p := staged[0]
		if p.Original != initialContent {
			t.Errorf("expected Original %q, got %q", initialContent, p.Original)
		}
		if p.Mode != fileMode {
			t.Errorf("expected Mode %v, got %v", fileMode, p.Mode)
		}
		if p.IsNew {
			t.Errorf("expected IsNew to be false for existing file, got true")
		}
	})

	t.Run("non-existent file staged sets IsNew and leaves original empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "non_existent.go")

		ledger := NewLedger()
		patch := Patch{
			FilePath:   testPath,
			NewContent: "package newpkg\n",
		}

		if err := ledger.Stage(testPath, patch); err != nil {
			t.Fatalf("Stage failed: %v", err)
		}

		staged := ledger.GetStaged()
		if len(staged) != 1 {
			t.Fatalf("expected 1 staged patch, got %d", len(staged))
		}

		p := staged[0]
		if p.Original != "" {
			t.Errorf("expected Original to be empty, got %q", p.Original)
		}
		if !p.IsNew {
			t.Errorf("expected IsNew to be true for new file, got false")
		}
	})

	t.Run("explicit original content is preserved even if file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "explicit.go")
		diskContent := "disk content"
		explicitOriginal := "explicit original"
		fileMode := os.FileMode(0600)

		if err := os.WriteFile(testPath, []byte(diskContent), fileMode); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		ledger := NewLedger()
		patch := Patch{
			FilePath:   testPath,
			Original:   explicitOriginal,
			NewContent: "new content",
		}

		if err := ledger.Stage(testPath, patch); err != nil {
			t.Fatalf("Stage failed: %v", err)
		}

		staged := ledger.GetStaged()
		if len(staged) != 1 {
			t.Fatalf("expected 1 staged patch, got %d", len(staged))
		}

		p := staged[0]
		if p.Original != explicitOriginal {
			t.Errorf("expected Original %q, got %q", explicitOriginal, p.Original)
		}
		if p.IsNew {
			t.Errorf("expected IsNew to be false, got true")
		}
	})

	t.Run("unexpected stat error returns error immediately", func(t *testing.T) {
		tmpDir := t.TempDir()
		inaccessibleDir := filepath.Join(tmpDir, "inaccessible")
		if err := os.MkdirAll(inaccessibleDir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		testPath := filepath.Join(inaccessibleDir, "file.go")

		// Remove search/read permissions from parent directory to trigger EACCES on Stat
		if err := os.Chmod(inaccessibleDir, 0000); err != nil {
			t.Fatalf("failed to chmod dir: %v", err)
		}
		defer func() {
			_ = os.Chmod(inaccessibleDir, 0755)
		}()

		ledger := NewLedger()
		patch := Patch{
			FilePath:   testPath,
			NewContent: "new content",
		}

		err := ledger.Stage(testPath, patch)
		if err == nil {
			t.Fatalf("expected Stage to fail on inaccessible stat path, got nil")
		}
	})

	t.Run("unexpected read error returns error immediately", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "unreadable.go")
		if err := os.WriteFile(testPath, []byte("secret content"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
		// Make file unreadable to trigger EACCES on ReadFile
		if err := os.Chmod(testPath, 0000); err != nil {
			t.Fatalf("failed to chmod file: %v", err)
		}
		defer func() {
			_ = os.Chmod(testPath, 0644)
		}()

		ledger := NewLedger()
		patch := Patch{
			FilePath:   testPath,
			NewContent: "new content",
		}

		err := ledger.Stage(testPath, patch)
		if err == nil {
			t.Fatalf("expected Stage to fail on unreadable file, got nil")
		}
	})
}

func TestLedger_DeterministicStagedOrdering(t *testing.T) {
	ledger := NewLedger()
	files := []string{"z_file.go", "a_file.go", "m_file.go", "b_file.go", "k_file.go"}
	for _, f := range files {
		ledger.Staged[f] = Patch{FilePath: f, NewContent: "content"}
	}

	expectedOrder := []string{"a_file.go", "b_file.go", "k_file.go", "m_file.go", "z_file.go"}
	for i := 0; i < 30; i++ {
		plan := ledger.buildCommitPlan()
		if len(plan.Prepare) != len(expectedOrder) {
			t.Fatalf("iteration %d: expected %d prepare steps, got %d", i, len(expectedOrder), len(plan.Prepare))
		}
		for j, step := range plan.Prepare {
			expectedStepID := expectedOrder[j] + "_prepare"
			if step.ID() != expectedStepID {
				t.Fatalf("iteration %d: expected prepare step %d to be %s, got %s", i, j, expectedStepID, step.ID())
			}
		}
		for j, step := range plan.Apply {
			expectedStepID := expectedOrder[j] + "_apply"
			if step.ID() != expectedStepID {
				t.Fatalf("iteration %d: expected apply step %d to be %s, got %s", i, j, expectedStepID, step.ID())
			}
		}
	}
}


