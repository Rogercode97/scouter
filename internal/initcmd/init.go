package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const hookIdentifier = "scouter-rewrite.sh"

// Run installs the scouter integration for a specific agent.
func Run(args []string) error {
	agent := "claude" // Default for backward compatibility with 'init'
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		agent = args[0]
	}

	for _, arg := range args {
		if arg == "--uninstall" {
			return Uninstall(agent)
		}
	}

	// Always ensure filter directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	filterDir := filepath.Join(home, ".config", "scouter", "filters")
	if err := os.MkdirAll(filterDir, 0755); err != nil {
		return fmt.Errorf("create filter dir: %w", err)
	}

	switch agent {
	case "claude":
		return installClaude(home, filterDir)
	case "gemini-cli":
		return installGeminiCLI(home)
	case "opencode":
		return installOpenCode(home)
	default:
		return fmt.Errorf("unknown agent: %s (supported: claude, gemini-cli, opencode)", agent)
	}
}

func installClaude(home, filterDir string) error {
	// 1. Write hook script
	hookDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("create hook dir: %w", err)
	}

	hookPath := filepath.Join(hookDir, hookIdentifier)
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	// 2. Patch settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchClaudeSettings(settingsPath, hookPath); err != nil {
		return fmt.Errorf("patch settings: %w", err)
	}

	fmt.Println("✅ Scouter init complete (Claude Code):")
	fmt.Printf("  hook: %s\n", hookPath)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	return nil
}

func installGeminiCLI(home string) error {
	configPath := filepath.Join(home, ".gemini", "settings.json")
	binPath, err := os.Executable()
	if err != nil {
		binPath, _ = filepath.Abs(os.Args[0])
	}

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp"},
		"trust":   true,
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("✅ Scouter integrated with Gemini CLI (MCP)!\n")
	fmt.Printf("  settings: %s\n", configPath)
	return nil
}

func installOpenCode(home string) error {
	configPath := filepath.Join(home, ".config", "opencode", "settings.json")
	binPath, err := os.Executable()
	if err != nil {
		binPath, _ = filepath.Abs(os.Args[0])
	}

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"type":    "local",
		"command": []string{binPath, "mcp"},
		"enabled": true,
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("✅ Scouter integrated with OpenCode (MCP)!\n")
	fmt.Printf("  settings: %s\n", configPath)
	return nil
}

// Uninstall removes scouter integration.
func Uninstall(agent string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	switch agent {
	case "claude":
		hookPath := filepath.Join(home, ".claude", "hooks", hookIdentifier)
		_ = os.Remove(hookPath)
		settingsPath := filepath.Join(home, ".claude", "settings.json")
		unpatchClaudeSettings(settingsPath)
		fmt.Println("✅ Scouter uninstalled from Claude")
	case "gemini-cli":
		settingsPath := filepath.Join(home, ".gemini", "settings.json")
		removeMCPServer(settingsPath, "scouter")
		fmt.Println("✅ Scouter removed from Gemini CLI")
	case "opencode":
		settingsPath := filepath.Join(home, ".config", "opencode", "settings.json")
		removeMCPServer(settingsPath, "scouter")
		fmt.Println("✅ Scouter removed from OpenCode")
	}

	return nil
}

func removeMCPServer(path, name string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return
	}
	delete(mcpServers, name)
	newData, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(path, newData, 0600)
}

// patchClaudeSettings adds the scouter hook to Claude Code settings.json.
func patchClaudeSettings(path, hookPath string) error {
	var settings map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]any)
		} else {
			return fmt.Errorf("read settings: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	scouterHookEntry := map[string]any{
		"type":    "command",
		"command": hookPath,
	}

	scouterMatcher := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{scouterHookEntry},
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	found := false
	for i, entry := range preToolUse {
		if isScouterEntry(entry) {
			preToolUse[i] = scouterMatcher
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, scouterMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0600)
}

func unpatchClaudeSettings(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return
	}

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return
	}
	arr, ok := existing.([]any)
	if !ok {
		return
	}

	var filtered []any
	for _, entry := range arr {
		if !isScouterEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, out, 0600)
}

func isScouterEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, hookIdentifier) {
			return true
		}
	}
	return false
}

// Claude Code PreToolUse hook script
const hookScript = `#!/bin/bash
# scouter — CLI Token Killer hook for Claude Code
# PreToolUse hook: reads JSON from stdin, rewrites command through scouter

# Graceful degradation: if scouter or jq are missing, allow original command
if ! command -v scouter &>/dev/null || ! command -v jq &>/dev/null; then
  exit 0
fi

set -euo pipefail

leading_ws_len() {
  local input="$1"
  local len=${#input}
  local i=0

  while [ $i -lt $len ]; do
    case "${input:$i:1}" in
      [[:space:]]) i=$((i + 1)) ;;
      *) break ;;
    esac
  done

  printf '%s' "$i"
}

trailing_ws_len() {
  local input="$1"
  local i=$((${#input} - 1))
  local count=0

  while [ $i -ge 0 ]; do
    case "${input:$i:1}" in
      [[:space:]])
        count=$((count + 1))
        i=$((i - 1))
        ;;
      *) break ;;
    esac
  done

  printf '%s' "$count"
}

extract_first_segment() {
  local input="$1"
  local len=${#input}
  local i=0
  local quote=""
  local ch

  while [ $i -lt $len ]; do
    ch="${input:$i:1}"

    if [ -n "$quote" ]; then
      if [ "$ch" = "\\" ] && [ "$quote" = '"' ]; then
        i=$((i + 2))
        continue
      fi

      if [ "$ch" = "$quote" ]; then
        quote=""
      fi

      i=$((i + 1))
      continue
    fi

    case "$ch" in
      "'") quote="'" ;;
      '"') quote='"' ;;
      ';'|'|'|'&') break ;;
    esac

    i=$((i + 1))
  done

  printf '%s' "${input:0:i}"
}

INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

if [ -z "$CMD" ]; then
  exit 0
fi

FIRST_LINE=$(printf '%s\n' "$CMD" | head -1)
FIRST_SEGMENT=$(extract_first_segment "$FIRST_LINE")
LEADING_WS_LEN=$(leading_ws_len "$FIRST_SEGMENT")
TRAILING_WS_LEN=$(trailing_ws_len "$FIRST_SEGMENT")
FIRST_CMD_LEN=$((${#FIRST_SEGMENT} - LEADING_WS_LEN - TRAILING_WS_LEN))
if [ $FIRST_CMD_LEN -lt 0 ]; then
  FIRST_CMD_LEN=0
fi
FIRST_PREFIX="${FIRST_SEGMENT:0:LEADING_WS_LEN}"
FIRST_CMD="${FIRST_SEGMENT:LEADING_WS_LEN:FIRST_CMD_LEN}"
FIRST_SUFFIX_START=$((LEADING_WS_LEN + FIRST_CMD_LEN))
FIRST_SUFFIX="${FIRST_SEGMENT:FIRST_SUFFIX_START}"

case "$FIRST_CMD" in
  scouter\ *|*/scouter\ *) exit 0 ;;
esac

ENV_PREFIX=$(printf '%s' "$FIRST_CMD" | sed -E 's/^(([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]+[[:space:]]*)*).*/\1/')
BARE_CMD="${FIRST_CMD:${#ENV_PREFIX}}"
BASE=$(echo "$BARE_CMD" | awk '{print $1}')

REWRITE=""
case "$BASE" in
  git|go|cargo|npm|npx|yarn|pnpm|docker|kubectl|make|pip|pytest|jest|tsc|eslint|rustc)
    REST="${CMD:${#FIRST_SEGMENT}}"
    REWRITE="${FIRST_PREFIX}${ENV_PREFIX}scouter -- ${BARE_CMD}${FIRST_SUFFIX}${REST}"
    ;;
esac

if [ -z "$REWRITE" ]; then
  exit 0
fi

ORIGINAL_INPUT=$(echo "$INPUT" | jq -c '.tool_input')
UPDATED_INPUT=$(echo "$ORIGINAL_INPUT" | jq --arg cmd "$REWRITE" '.command = $cmd')

jq -n \
  --argjson updated "$UPDATED_INPUT" \
  '{
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "allow",
      "permissionDecisionReason": "scouter auto-rewrite",
      "updatedInput": $updated
    }
  }'
`
