package engine_test

import (
	"context"
	"errors"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type mockMessenger struct {
	response string
	err      error
}

func (m *mockMessenger) Ask(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return m.response, m.err
}

func TestEvolutionEngine_ProposeEvolution(t *testing.T) {
	tests := []struct {
		name        string
		proposal    string
		force       bool
		messenger   engine.Messenger
		wantResult  string
		wantErr     bool
		errContains string
		wantPatches []engine.Patch
	}{
		{
			name:     "successful evolution",
			proposal: "add a new function",
			force:    false,
			messenger: &mockMessenger{
				response: `[{"file": "test_file.go", "content": "func newFunc() {}"}]`,
				err:      nil,
			},
			wantResult: "Evolution staged in Ledger for 1 files",
			wantErr:    false,
			wantPatches: []engine.Patch{
				{
					FilePath:   filepath.Join("test_file.go"), // Use filepath.Join to normalize paths for tests
					NewContent: "func newFunc() {}",
				},
			},
		},
		{
			name:     "messenger error",
			proposal: "cause error",
			force:    false,
			messenger: &mockMessenger{
				response: "",
				err:      errors.New("messenger timeout"),
			},
			wantResult:  "",
			wantErr:     true,
			errContains: "sampling evolution failed: messenger timeout",
			wantPatches: nil,
		},
		{
			name:     "malformed json",
			proposal: "bad json",
			force:    false,
			messenger: &mockMessenger{
				response: `this is not json`,
				err:      nil,
			},
			wantResult:  "",
			wantErr:     true,
			errContains: "failed to parse mutation JSON",
			wantPatches: nil,
		},
		{
			name:     "sovereignty violation without force",
			proposal: "lobotomize",
			force:    false,
			messenger: &mockMessenger{
				response: `[{"file": "internal/mcp/handlers.go", "content": "package mcp\nfunc Handlers(){}"}]`,
				err:      nil,
			},
			wantResult:  "",
			wantErr:     true,
			errContains: "SOVEREIGNTY VIOLATION",
			wantPatches: nil,
		},
		{
			name:     "sovereignty violation with force",
			proposal: "lobotomize but forced",
			force:    true,
			messenger: &mockMessenger{
				response: `[{"file": "internal/mcp/handlers.go", "content": "package mcp\nfunc Handlers(){}"}]`,
				err:      nil,
			},
			wantResult: "Evolution staged in Ledger for 1 files",
			wantErr:    false,
			wantPatches: []engine.Patch{
				{
					FilePath:   filepath.Join("internal", "mcp", "handlers.go"),
					NewContent: "package mcp\nfunc Handlers(){}",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := engine.NewLedger()
			evo := engine.NewEvolutionEngine(nil, ledger, nil)
			res, err := evo.ProposeEvolution(context.Background(), tt.proposal, tt.force, tt.messenger)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ProposeEvolution() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ProposeEvolution() error = %q, want string containing %q", err, tt.errContains)
			}

			if !tt.wantErr && !strings.Contains(res, tt.wantResult) {
				t.Errorf("ProposeEvolution() result = %q, want string containing %q", res, tt.wantResult)
			}

			// Validate Ledger Patches
			gotPatches := ledger.GetStaged()

			if len(gotPatches) == 0 && len(tt.wantPatches) == 0 {
				return
			}

			// Fix expected paths to match utils.ValidatePath behavior
			for i := range tt.wantPatches {
				absPath, err := utils.ValidatePath(tt.wantPatches[i].FilePath)
				if err != nil {
					t.Fatalf("Failed to validate expected path: %v", err)
				}
				tt.wantPatches[i].FilePath = absPath
			}

			// Ignore Original and Diff fields for testing simplicity unless explicitly needed
			opts := []cmp.Option{
				cmpopts.IgnoreFields(engine.Patch{}, "Original", "Diff"),
			}
			if diff := cmp.Diff(tt.wantPatches, gotPatches, opts...); diff != "" {
				t.Errorf("Ledger patches mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type mockStrategy struct {
	tasks []engine.PropagationTask
	err   error
}

func (m *mockStrategy) Discover(ctx context.Context, startSymbol string, depth int) iter.Seq2[engine.PropagationTask, error] {
	return func(yield func(engine.PropagationTask, error) bool) {
		if m.err != nil {
			yield(engine.PropagationTask{}, m.err)
			return
		}
		for _, task := range m.tasks {
			if !yield(task, nil) {
				return
			}
		}
	}
}

func TestEvolutionEngine_Propagate(t *testing.T) {
	tests := []struct {
		name        string
		strategy    *mockStrategy
		messenger   engine.Messenger
		wantResult  string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful propagation",
			strategy: &mockStrategy{
				tasks: []engine.PropagationTask{},
				err:   nil,
			},
			messenger:  nil,
			wantResult: "Transformation staged in Ledger for 0 files",
			wantErr:    false,
		},
		{
			name: "propagation error",
			strategy: &mockStrategy{
				tasks: nil,
				err:   errors.New("strategy failed"),
			},
			messenger:   nil,
			wantResult:  "",
			wantErr:     true,
			errContains: "strategy failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := engine.NewLedger()
			ripple := &engine.RippleEngine{
				Strategy: tt.strategy,
				// Provide dummy validator slice to prevent nil dereference if ripple checks it
				Validators: []engine.Validator{},
			}
			evo := engine.NewEvolutionEngine(nil, ledger, ripple)

			res, err := evo.Propagate(context.Background(), "TargetSymbol", "transform rule", tt.messenger)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Propagate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Propagate() error = %v, want string containing %q", err, tt.errContains)
			}
			if !tt.wantErr && !strings.Contains(res, tt.wantResult) {
				t.Errorf("Propagate() result = %q, want string containing %q", res, tt.wantResult)
			}
		})
	}
}

func TestEvolutionEngine_CommitAndRollbackLedger(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_file.go")
	originalContent := "package main\nfunc main() {}"
	newContent := "package main\nfunc main() { println() }"

	// 1. Setup file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ledger := engine.NewLedger()
	evo := engine.NewEvolutionEngine(nil, ledger, nil)

	// Test uninitialized / empty ledger
	res, err := evo.CommitLedger(context.Background())
	if err != nil {
		t.Errorf("CommitLedger with empty ledger should return nil error, got %v", err)
	}
	if !strings.Contains(res, "No changes staged") {
		t.Errorf("Expected No changes staged, got %q", res)
	}

	// 2. Stage mutation
	err = evo.StageMutation(context.Background(), testFile, newContent)
	if err != nil {
		t.Fatalf("StageMutation failed: %v", err)
	}

	// Verify Rollback
	res, err = evo.RollbackLedger(context.Background())
	if err != nil {
		t.Fatalf("RollbackLedger failed: %v", err)
	}
	if !strings.Contains(res, "Ledger rolled back") {
		t.Errorf("Expected rollback success message, got %q", res)
	}
	if len(ledger.GetStaged()) != 0 {
		t.Errorf("Expected 0 staged files after rollback, got %d", len(ledger.GetStaged()))
	}

	// 3. Stage again for commit
	err = evo.StageMutation(context.Background(), testFile, newContent)
	if err != nil {
		t.Fatalf("StageMutation failed: %v", err)
	}

	// 4. Commit
	res, err = evo.CommitLedger(context.Background())
	if err != nil {
		t.Fatalf("CommitLedger failed: %v", err)
	}
	if !strings.Contains(res, "Committed changes") {
		t.Errorf("Expected commit success message, got %q", res)
	}
	if len(ledger.GetStaged()) != 0 {
		t.Errorf("Expected ledger to be cleared after commit, got %d", len(ledger.GetStaged()))
	}

	// Verify file content changed
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after commit: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("Commit failed to write new content, got %q, want %q", string(content), newContent)
	}
}
