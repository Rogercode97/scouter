package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
)

// --- Active Healing (Autonomous) ---

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
