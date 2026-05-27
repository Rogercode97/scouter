package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDDExplorer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "sdd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create dummy artifacts
	os.MkdirAll("openspec/specs", 0755)
	os.WriteFile("openspec/state.yaml", []byte("phase: Test\ntrajectory: [Step1]\nactive_changes: [Change1]"), 0644)
	os.WriteFile("openspec/tasks.md", []byte("# Tasks\n- [x] Task 1\n- [ ] Task 2"), 0644)
	os.WriteFile("openspec/specs/test_spec.md", []byte("Content of test spec"), 0644)

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	// Unlock heavy arsenal to access scouter_sdd
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "scouter_unlock",
	})
	if err != nil {
		t.Fatalf("failed to unlock arsenal: %v", err)
	}

	t.Run("Explore roadmap", func(t *testing.T) {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "scouter_sdd",
			Arguments: map[string]any{
				"type": "roadmap",
			},
		})
		if err != nil {
			t.Fatalf("scouter_sdd roadmap failed: %v", err)
		}
		if res.IsError {
			t.Fatalf("expected no error, got error content: %v", res.Content[0].(*mcp.TextContent).Text)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !json.Valid([]byte(text[strings.Index(text, "}")+1:])) {
			// Try finding where JSON starts
			jsonIdx := strings.Index(text, "{")
			if jsonIdx == -1 {
				t.Fatalf("no JSON found in output: %s", text)
			}
			var roadmap engine.SDDRoadmap
			if err := json.Unmarshal([]byte(text[jsonIdx:]), &roadmap); err != nil {
				t.Fatalf("failed to unmarshal roadmap: %v\nText: %s", err, text)
			}
			if roadmap.Phase != "Test" {
				t.Errorf("expected phase Test, got %s", roadmap.Phase)
			}
		}
	})

	t.Run("Explore tasks", func(t *testing.T) {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "scouter_sdd",
			Arguments: map[string]any{
				"type": "tasks",
			},
		})
		if err != nil {
			t.Fatalf("scouter_sdd tasks failed: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		jsonIdx := strings.Index(text, "[")
		if jsonIdx == -1 {
			t.Fatalf("no JSON list found in output: %s", text)
		}
		var tasks []engine.SDDTask
		if err := json.Unmarshal([]byte(text[jsonIdx:]), &tasks); err != nil {
			t.Fatalf("failed to unmarshal tasks: %v", err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
	})

	t.Run("Explore specs", func(t *testing.T) {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "scouter_sdd",
			Arguments: map[string]any{
				"type":  "specs",
				"query": "test",
			},
		})
		if err != nil {
			t.Fatalf("scouter_sdd specs failed: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		jsonIdx := strings.Index(text, "[")
		if jsonIdx == -1 {
			t.Fatalf("no JSON list found in output: %s", text)
		}
		var specs []engine.SpecResult
		if err := json.Unmarshal([]byte(text[jsonIdx:]), &specs); err != nil {
			t.Fatalf("failed to unmarshal specs: %v", err)
		}
		if len(specs) == 0 {
			t.Errorf("expected at least one spec, got 0")
		}
	})
}
