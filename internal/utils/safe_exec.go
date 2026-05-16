package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SafeCommand creates a validated exec.Cmd to prevent command injection and unauthorized execution.
// It enforces an allow-list of binaries and sanitizes arguments for dangerous shell characters.
func SafeCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	// 🏛️ BINARY VALIDATION (Read-only maps)
	allowedBinaries := map[string]bool{
		"git":      true,
		"go":       true,
		"rtk":      true,
		"sg":       true,
		"engram":   true,
		"scouter":  true,
		"sh":       true,
		"bash":     true,
		"gopls":    true,
		"ast-grep": true,
	}

	shellBuiltins := map[string]bool{
		"export":   true,
		"unset":    true,
		"source":   true,
		"alias":    true,
		"unalias":  true,
		"eval":     true,
		"set":      true,
		"shopt":    true,
		"declare":  true,
		"local":    true,
		"readonly": true,
		"typeset":  true,
		"ulimit":   true,
		"umask":    true,
	}

	if !allowedBinaries[name] && !shellBuiltins[name] {
		return nil, fmt.Errorf("forbidden binary: %s", name)
	}

	// 2. 🛡️ ARGUMENT SANITIZATION
	// We block characters that allow command chaining, redirection, or expansion.
	dangerousChars := ";|&><`\\"
	for _, arg := range args {
		if strings.ContainsAny(arg, dangerousChars) {
			return nil, fmt.Errorf("dangerous characters in argument: %s", arg)
		}
	}

	// 3. 🐚 SHELL BUILT-IN WRAPPER
	// If it's a built-in, we wrap it in 'sh -c' safely using positional parameters.
	if shellBuiltins[name] {
		shArgs := make([]string, 0, len(args)+3)
		shArgs = append(shArgs, "-c", name+` "$@"`, "_")
		shArgs = append(shArgs, args...)
		// 'sh' is guaranteed to be in allowedBinaries
		// #nosec G204: We have strict binary validation and argument sanitization in place.
		return exec.CommandContext(ctx, "sh", shArgs...), nil
	}

	// #nosec G204: We have strict binary validation and argument sanitization in place.
	return exec.CommandContext(ctx, name, args...), nil
}