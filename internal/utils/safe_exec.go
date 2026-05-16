package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

var allowedBinaries = map[string]bool{
	"git":    true,
	"go":     true,
	"rtk":    true,
	"engram": true,
	"sg":     true,
	"sh":     true,
	"bash":   true,
	"scouter": true,
}

// SafeCommand creates an exec.Cmd after validating the binary against an allow-list
// and checking for potentially dangerous argument patterns.
func SafeCommand(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
	// 1. Validate binary
	binary := name
	if strings.Contains(binary, "/") {
		// If it's a path, check the base name or ensure it's a relative path to bin/
		parts := strings.Split(binary, "/")
		binary = parts[len(parts)-1]
	}

	if !allowedBinaries[binary] {
		return nil, fmt.Errorf("forbidden binary: %s", name)
	}

	// 2. Validate arguments for common injection patterns
	// Even though exec.Command is generally safe (not a shell), 
	// we want to prevent unintended behavior or future regressions.
	for _, a := range arg {
		// Block common shell metacharacters that shouldn't be in individual arguments
		// unless specifically required (like in 'sh -c').
		if name != "sh" && name != "bash" {
			if strings.ContainsAny(a, "`$()<>|;&") {
				return nil, fmt.Errorf("dangerous characters in argument: %s", a)
			}
		}
	}

	return exec.CommandContext(ctx, name, arg...), nil
}
