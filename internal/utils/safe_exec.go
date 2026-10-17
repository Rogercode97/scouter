package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// allowedBinaries is the closed set of external programs SafeCommand will
// resolve. It deliberately excludes shells and code-evaluation builtins:
// allowing "sh -c" or "eval" reintroduces shell parsing and turns this wrapper
// into a pass-through for arbitrary execution.
var allowedBinaries = map[string]bool{
	"git":      true,
	"go":       true,
	"rtk":      true,
	"sg":       true,
	"scouter":  true,
	"gopls":    true,
	"ast-grep": true,
}

// dangerousChars block command chaining, redirection and expansion. This
// denylist does NOT stop argument injection through leading-dash flags
// (CWE-88); that gap is open and tracked in odd/tasks/safecommand-hardening.md.
const dangerousChars = ";|&><`\\"

// SafeCommand creates a validated *exec.Cmd from the allow-listed binaries.
// It rejects any binary outside allowedBinaries and any argument containing a
// character from dangerousChars. It never wraps its input in a shell.
func SafeCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	if !allowedBinaries[name] {
		return nil, fmt.Errorf("forbidden binary: %s", name)
	}

	for _, arg := range args {
		if strings.ContainsAny(arg, dangerousChars) {
			return nil, fmt.Errorf("dangerous characters in argument: %s", arg)
		}
	}

	// #nosec G204 -- name is validated against the closed allowedBinaries set
	// above and each argument is checked against dangerousChars. No shell is
	// involved, so os/exec passes the argv vector directly.
	return exec.CommandContext(ctx, name, args...), nil
}
