package utils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// AllowedCommands is the strict allow-list for Scouter's execution engine.
var AllowedCommands = map[string]bool{
	"git":      true,
	"engram":   true,
	"rtk":      true,
	"sg":       true,
	"ast-grep": true,
	"bash":     true,
	"go":       true,
}

// SafeCommand creates a wrapped exec.CommandContext that enforces a strict allow-list
// to prevent arbitrary command execution (CWE-78).
func SafeCommand(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
	baseName := filepath.Base(name)
	if !AllowedCommands[baseName] {
		return nil, fmt.Errorf("security violation: command '%s' is not in the allow-list", name)
	}
	return exec.CommandContext(ctx, name, arg...), nil
}
