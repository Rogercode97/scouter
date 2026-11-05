package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readGentleMCP(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "mcp.json"))
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse mcp.json: %v", err)
	}
	return config
}

func scouterServer(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found")
	}
	srv, ok := servers["scouter"].(map[string]any)
	if !ok {
		t.Fatal("scouter server not found")
	}
	return srv
}

func TestInstallGentleShellNew(t *testing.T) {
	home := t.TempDir()
	if err := installGentleShell(home); err != nil {
		t.Fatalf("install: %v", err)
	}
	srv := scouterServer(t, readGentleMCP(t, home))
	args, ok := srv["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "mcp" {
		t.Fatalf("args = %v, want [mcp]", srv["args"])
	}
	if _, ok := srv["command"].(string); !ok {
		t.Fatalf("command = %v, want string", srv["command"])
	}
}

func TestInstallGentleShellPreservesOthers(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{"mcpServers": {"other": {"command": "other", "args": []}}}`
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(existing), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := installGentleShell(home); err != nil {
		t.Fatalf("install: %v", err)
	}
	config := readGentleMCP(t, home)
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("existing server lost")
	}
	scouterServer(t, config)
}
