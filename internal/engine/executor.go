package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
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

// drainGrace bounds how long a cancelled command may keep its readers waiting on
// a pipe an inherited grandchild still holds. It applies to cancellation only:
// on normal completion the readers see EOF and drain in full, never bounded.
// It is a var so a test can shrink it instead of sleeping for seconds.
var drainGrace = 2 * time.Second

// Execute runs a command, capturing stdout and stderr concurrently.
func Execute(ctx context.Context, command string, args []string) (*Result, error) {
	start := time.Now()

	cmd := makeCommand(ctx, command, args)
	// Don't connect stdin for captured commands — prevents blocking on
	// commands that don't read stdin (most filtered commands).
	// Passthrough commands still get stdin via the Passthrough function.

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = stdoutBuf.ReadFrom(stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = stderrBuf.ReadFrom(stderrPipe)
	}()

	// The readers must be finished before Wait: Wait closes the pipes, and data
	// they had not consumed would be lost silently. Cancellation is the one case
	// where waiting could last forever, so it gets the bounded close below.
	readersDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			begin := time.NewTimer(drainGrace)
			defer begin.Stop()
			select {
			case <-readersDone:
			case <-begin.C:
				// A surviving grandchild still holds the write ends. Close the
				// read ends so the readers unblock and the command is reaped.
				_ = stdoutPipe.Close()
				_ = stderrPipe.Close()
			}
		case <-readersDone:
		}
	}()

	wg.Wait()
	close(readersDone)

	err = cmd.Wait()
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
