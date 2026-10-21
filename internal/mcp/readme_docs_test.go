package mcp

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const readmePath = "../../README.md"

func readReadmeFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	return string(data)
}

// mcpSection isolates the README part that documents the MCP surface.
func mcpSection(t *testing.T, readme string) string {
	t.Helper()
	const marker = "### Model Context Protocol"
	start := strings.Index(readme, marker)
	if start < 0 {
		t.Fatal("README no longer documents the Model Context Protocol surface")
	}
	section := readme[start:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

var (
	groupHeaderRe  = regexp.MustCompile(`(?m)^\*\*(.+?)\*\*\s*$`)
	documentedName = regexp.MustCompile("`([a-z][a-z0-9_]*)`")
)

// readmeGroup is a bold-titled group of the MCP section and the identifiers
// it documents.
type readmeGroup struct {
	title string
	names []string
}

// parseGroups splits the MCP section into its bold-titled groups.
func parseGroups(section string) []readmeGroup {
	var groups []readmeGroup
	current := -1

	for _, line := range strings.Split(section, "\n") {
		if m := groupHeaderRe.FindStringSubmatch(line); m != nil {
			groups = append(groups, readmeGroup{title: strings.TrimSpace(m[1])})
			current = len(groups) - 1
			continue
		}
		if current < 0 {
			continue
		}
		for _, m := range documentedName.FindAllStringSubmatch(line, -1) {
			groups[current].names = append(groups[current].names, m[1])
		}
	}

	return groups
}

// TestREADME_MCPSurfaceMatchesServer proves that the README documents the MCP
// surface the server actually registers.
//
// A README that names a tool the server does not expose sends an agent to a
// call that fails at runtime; a README that omits a registered tool hides a
// capability. The claimed tool count must be the real one.
func TestREADME_MCPSurfaceMatchesServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	toolList, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}
	registeredTools := make(map[string]bool, len(toolList.Tools))
	for _, tool := range toolList.Tools {
		registeredTools[tool.Name] = true
	}

	promptList, err := session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		t.Fatalf("failed to list prompts: %v", err)
	}
	registeredPrompts := make(map[string]bool, len(promptList.Prompts))
	for _, prompt := range promptList.Prompts {
		registeredPrompts[prompt.Name] = true
	}

	section := mcpSection(t, readReadmeFile(t))
	groups := parseGroups(section)
	if len(groups) == 0 {
		t.Fatal("README MCP section has no documented groups")
	}

	documentedTools := make(map[string]bool)
	documentedPrompts := make(map[string]bool)

	for _, group := range groups {
		isPromptGroup := strings.Contains(strings.ToLower(group.title), "prompt")

		for _, name := range group.names {
			if isPromptGroup {
				if !registeredPrompts[name] {
					t.Errorf("README documents %q as an MCP prompt, but the server registers no such prompt", name)
				}
				documentedPrompts[name] = true
				continue
			}

			if !registeredTools[name] {
				t.Errorf("README documents %q as an MCP tool, but the server registers no such tool", name)
			}
			documentedTools[name] = true
		}
	}

	for name := range registeredTools {
		if !documentedTools[name] {
			t.Errorf("MCP tool %q is registered but not documented in the README", name)
		}
	}

	for name := range registeredPrompts {
		if !documentedPrompts[name] && !documentedTools[name] {
			t.Errorf("MCP prompt %q is registered but not documented in the README", name)
		}
	}

	claim := regexp.MustCompile(`exposes (\d+) tools`).FindStringSubmatch(section)
	if claim == nil {
		t.Fatal("README MCP section does not state how many tools the server exposes")
	}
	claimed, err := strconv.Atoi(claim[1])
	if err != nil {
		t.Fatalf("unparsable tool count %q: %v", claim[1], err)
	}
	if claimed != len(registeredTools) {
		t.Errorf("README claims the MCP server exposes %d tools, but it registers %d", claimed, len(registeredTools))
	}

	promptClaim := regexp.MustCompile(`plus (\d+) prompts`).FindStringSubmatch(section)
	if promptClaim == nil {
		t.Fatal("README MCP section does not state how many prompts the server registers")
	}
	claimedPrompts, err := strconv.Atoi(promptClaim[1])
	if err != nil {
		t.Fatalf("unparsable prompt count %q: %v", promptClaim[1], err)
	}
	if claimedPrompts != len(registeredPrompts) {
		t.Errorf("README claims the MCP server registers %d prompts, but it registers %d", claimedPrompts, len(registeredPrompts))
	}
}
