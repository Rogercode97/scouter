package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// binaryPolicy is what SafeCommand needs to know about one allow-listed
// program: which flags it may be invoked with. Flags are matched by name, so
// "--unified" permits both "--unified" and "--unified=3".
type binaryPolicy struct {
	flags map[string]bool
}

func flagSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// allowedBinaries is the closed set of external programs SafeCommand will
// resolve, each with the flags it may be called with. It deliberately excludes
// shells and code-evaluation builtins: allowing "sh -c" or "eval" reintroduces
// shell parsing and turns this wrapper into a pass-through for arbitrary
// execution.
//
// The flag lists are not decoration. An allow-listed binary with free-form
// flags is still an execution route: `git diff --upload-pack=/bin/sh` runs a
// shell and `git diff --output=<path>` writes an arbitrary file, and neither
// carries a character the argument denylist catches. Every flag listed here
// exists because a production caller passes it; add one only together with the
// caller that needs it.
var allowedBinaries = map[string]binaryPolicy{
	"git":      {flags: flagSet("--unified")},
	"rtk":      {flags: flagSet("--pointer", "--ultra-compact")},
	"sg":       {flags: flagSet("--pattern", "--rewrite")},
	"ast-grep": {flags: flagSet("--pattern", "--rewrite")},
	"go":       {},
	"scouter":  {},
	"gopls":    {},
}

// dangerousChars block command chaining, redirection and expansion.
const dangerousChars = ";|&><`\\"

// SafeCommand creates a validated *exec.Cmd from the allow-listed binaries.
// It rejects any binary outside allowedBinaries, any argument carrying a
// character from dangerousChars, and any flag that is not allow-listed for that
// binary. It never wraps its input in a shell.
func SafeCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	policy, ok := allowedBinaries[name]
	if !ok {
		return nil, fmt.Errorf("forbidden binary: %s", name)
	}

	for _, arg := range args {
		if err := validateArg(name, policy, arg); err != nil {
			return nil, err
		}
	}

	// #nosec G204 -- name is validated against the closed allowedBinaries set
	// above and every argument passes validateArg. No shell is involved, so
	// os/exec passes the argv vector directly.
	return exec.CommandContext(ctx, name, args...), nil
}

// validateArg applies two independent gates to one argument. The character
// denylist stops chaining, redirection and expansion; the flag policy stops
// argument injection, where an allow-listed binary is asked to load code or
// write a file through an option nobody meant to expose.
func validateArg(binary string, policy binaryPolicy, arg string) error {
	if strings.ContainsAny(arg, dangerousChars) {
		return fmt.Errorf("dangerous characters in argument: %s", arg)
	}
	if !strings.HasPrefix(arg, "-") {
		return nil
	}
	// A bare "--" is end-of-options and carries no behaviour of its own.
	if arg == "--" {
		return nil
	}
	name, _, _ := strings.Cut(arg, "=")
	if !strings.HasPrefix(name, "--") {
		return fmt.Errorf("short flag not allowed for %s: %s", binary, arg)
	}
	if !policy.flags[name] {
		return fmt.Errorf("flag not allowed for %s: %s", binary, name)
	}
	return nil
}
