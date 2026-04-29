package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// --- Active Healing (Autonomous) ---

// HealerEngine manages the autonomous RCA -> Fix -> Verify loop.
type HealerEngine struct {
	store  store.Repository
	lspMgr *lsp.Manager
	
	// Bridge to MCP sampling
	DoFixRequest func(ctx context.Context, prompt string) (string, error)
}

func NewHealerEngine(s store.Repository, l *lsp.Manager) *HealerEngine {
	return &HealerEngine{
		store:  s,
		lspMgr: l,
	}
}

// Fix attempts to repair a test failure.
func (e *HealerEngine) Fix(ctx context.Context, errorLog string) (map[string]string, error) {
	// 1. RCA: Extract File and Line from log
	matches := filter.GoTestFailureRegex.FindStringSubmatch(errorLog)
	if len(matches) != 3 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}
	failingFileRaw := matches[1]
	lineNum, _ := strconv.Atoi(matches[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// 2. JIT Resolve Symbol and Context
	itPointers, _, err := StreamSymbols(ctx, failingFile)
	if err != nil {
		return nil, fmt.Errorf("jit parse failed: %w", err)
	}

	var target *types.ASTPointer
	for p := range itPointers {
		if lineNum >= p.StartLine && lineNum <= p.EndLine {
			target = &p
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("could not resolve symbol for %s:%d", failingFile, lineNum)
	}

	code, err := ReadFragment(ctx, failingFile, target.Range)
	if err != nil {
		return nil, fmt.Errorf("failed to read source context: %w", err)
	}

	// 3. Request Fix via Sampling
	prompt := fmt.Sprintf("Failing File: %s\nTarget Symbol: %s\nError Log:\n%s\n\nCurrent Code:\n%s", failingFile, target.Name, errorLog, code)
	newCodeRaw, err := e.DoFixRequest(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("sampling fix failed: %w", err)
	}
	newCode := utils.ExtractCodeBlock(newCodeRaw)

	// 4. Atomic Backup & Apply
	input, err := os.ReadFile(failingFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	backupFile := failingFile + ".bak"
	if err := os.WriteFile(backupFile, input, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}
	defer os.Remove(backupFile)

	updatedContent := string(input[:target.Range.Start]) + newCode + string(input[target.Range.End:])
	if err := os.WriteFile(failingFile, []byte(updatedContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to apply fix: %w", err)
	}

	// 5. Verify
	pkgDir := filepath.Dir(failingFile)
	root, _ := utils.GetRepoRoot()
	relPkgDir, _ := filepath.Rel(root, pkgDir)
	if relPkgDir == "" || relPkgDir == "." {
		relPkgDir = "./"
	} else {
		relPkgDir = "./" + relPkgDir
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-v", relPkgDir)
	testOut, testErr := cmd.CombinedOutput()
	
	status := "SUCCESS"
	if testErr != nil {
		status = "FAILED"
		// Restore
		_ = os.WriteFile(failingFile, input, 0644)
	}

	return map[string]string{
		"status":      status,
		"fixedCode":   newCode,
		"testOutput":  string(testOut),
		"failingFile": failingFile,
	}, nil
}

// --- Passive Health (Ingestion) ---

type TestResultStore interface {
	SaveTestResult(ctx context.Context, res *types.TestResult) error
}

type HealthEngine struct {
	store TestResultStore
}

func NewHealthEngine(store TestResultStore) *HealthEngine {
	return &HealthEngine{store: store}
}

func (h *HealthEngine) Ingest(ctx context.Context, r io.Reader) error {
	decoder := json.NewDecoder(r)
	buffers := make(map[string][]string)

	for decoder.More() {
		var event types.TestEvent
		if err := decoder.Decode(&event); err != nil {
			return fmt.Errorf("failed to decode TestEvent: %w", err)
		}

		if event.Test == "" {
			continue
		}
		key := event.Package + "." + event.Test

		switch event.Action {
		case "run":
			buffers[key] = []string{}
		case "output":
			buffers[key] = append(buffers[key], event.Output)
		case "pass", "fail", "skip":
			output := strings.Join(buffers[key], "")
			errorMessage, stackTrace := extractErrorAndStack(output)

			result := &types.TestResult{
				TestName:     event.Test,
				Status:       event.Action,
				ErrorMessage: errorMessage,
				StackTrace:   stackTrace,
				DurationMS:   int64(event.Elapsed * 1000),
				TargetSymbol: h.mapToSymbol(event.Test),
				Project:      event.Package,
			}
			if err := h.store.SaveTestResult(ctx, result); err != nil {
				return fmt.Errorf("failed to save TestResult: %w", err)
			}
			delete(buffers, key)
		}
	}
	return nil
}

func extractErrorAndStack(output string) (string, string) {
	lines := strings.Split(output, "\n")
	var errorMessage, stackTrace []string
	foundFail := false

	for _, line := range lines {
		if strings.Contains(line, "--- FAIL:") {
			foundFail = true
			continue
		}
		if foundFail {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Heuristic: First line with a colon after FAIL is the error message
			if strings.Contains(trimmed, ":") && len(errorMessage) == 0 {
				errorMessage = append(errorMessage, trimmed)
			}
			// All lines after FAIL are part of the stack trace/logs
			stackTrace = append(stackTrace, line)
		}
	}
	return strings.Join(errorMessage, "\n"), strings.Join(stackTrace, "\n")
}

func (h *HealthEngine) mapToSymbol(testName string) string {
	name := strings.TrimPrefix(testName, "Test")
	parts := strings.Split(name, "_")
	if len(parts) > 1 {
		if parts[0] == "Store" {
			return parts[0] + "." + parts[1]
		}
		return parts[0]
	}
	return name
}
