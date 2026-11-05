package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// shellBuiltins lists commands that are shell built-ins and cannot be
// executed directly via exec.Command.
var shellBuiltins = map[string]bool{
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

// makeCommand creates an exec.Cmd, wrapping shell built-ins with sh -c
// so they can be executed. Shell built-ins like "export" have no binary
// in $PATH and would fail with exec.Command directly.
func makeCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	if shellBuiltins[command] {
		shArgs := make([]string, 0, len(args)+3)
		shArgs = append(shArgs, "-c", command+` "$@"`, "_")
		shArgs = append(shArgs, args...)
		return exec.CommandContext(ctx, "sh", shArgs...)
	}
	return exec.CommandContext(ctx, command, args...)
}

// drainGrace bounds how long Wait keeps reading after the process exits or the
// context is cancelled, so a surviving grandchild that inherited the pipes
// cannot hold Execute open forever. It is not tuned down: the timer starts when
// the process exits, and a generous bound is what keeps a large already-buffered
// payload from being closed mid-copy. When it does expire the caller gets an
// error instead of a silent truncation.
const drainGrace = 2 * time.Second

// Execute runs a command, capturing stdout and stderr concurrently.
func Execute(ctx context.Context, command string, args []string) (*Result, error) {
	start := time.Now()

	cmd := makeCommand(ctx, command, args)
	// Don't connect stdin for captured commands — prevents blocking on
	// commands that don't read stdin (most filtered commands).
	// Passthrough commands still get stdin via the Passthrough function.

	// Handing os/exec the writers lets it own the copy goroutines, so Wait
	// drains both streams before returning and no ordering has to be gotten
	// right by hand. With StdoutPipe the reverse is true: Wait closes the pipes
	// and anything its readers had not consumed is lost silently.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.WaitDelay = drainGrace

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("wait command: %w", err)
		}
	}

	return &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

// Passthrough runs a command with inherited stdio (no capture).
func Passthrough(ctx context.Context, command string, args []string) (int, error) {
	cmd := makeCommand(ctx, command, args)
	cmd.Stdin = os.Stdin
	// When running as MCP server, we MUST NOT write to os.Stdout directly
	// as it will corrupt the JSON-RPC stream. Redirecting to Stderr is safer.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("passthrough: %w", err)
	}
	return 0, nil
}
