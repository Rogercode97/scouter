package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSDDEngine(t *testing.T) {
	// Setup temporary openspec directory
	tmpDir, err := os.MkdirTemp("", "openspec-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	openspecDir := filepath.Join(tmpDir, "openspec")
	os.MkdirAll(openspecDir, 0755)

	// Create dummy state.yaml
	stateContent := `
phase: Implementation
trajectory:
  - Discovery
  - Implementation
  - Verification
active_changes:
  - sdd-explorer
`
	os.WriteFile(filepath.Join(openspecDir, "state.yaml"), []byte(stateContent), 0644)

	// Create dummy tasks.md
	tasksContent := `
# Tasks

- [x] Task 1
- [ ] Task 2
`
	os.WriteFile(filepath.Join(openspecDir, "tasks.md"), []byte(tasksContent), 0644)

	// Create dummy spec
	specsDir := filepath.Join(openspecDir, "specs", "auth")
	os.MkdirAll(specsDir, 0755)
	specContent := "Spec for authentication"
	os.WriteFile(filepath.Join(specsDir, "spec.md"), []byte(specContent), 0644)

	engine := NewSDDEngine(tmpDir)

	t.Run("ParseRoadmap", func(t *testing.T) {
		roadmap, err := engine.ParseRoadmap(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if roadmap.Phase != "Implementation" {
			t.Errorf("expected phase Implementation, got %s", roadmap.Phase)
		}
		if len(roadmap.Trajectory) != 3 {
			t.Errorf("expected 3 trajectory steps, got %d", len(roadmap.Trajectory))
		}
	})

	t.Run("ParseTasks", func(t *testing.T) {
		tasks, err := engine.ParseTasks(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
		
		completedCount := 0
		for _, task := range tasks {
			if task.Completed {
				completedCount++
			}
		}
		if completedCount != 1 {
			t.Errorf("expected 1 completed task, got %d", completedCount)
		}
	})

	t.Run("SearchSpecs", func(t *testing.T) {
		results, err := engine.SearchSpecs(context.Background(), "auth", 10, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) == 0 {
			t.Errorf("expected at least one spec result, got 0")
		}
		found := false
		for _, res := range results {
			if filepath.Base(filepath.Dir(res.Path)) == "auth" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find auth spec, but didn't")
		}
	})
}
