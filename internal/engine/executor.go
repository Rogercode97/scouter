package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Rogercode97/scouter/internal/utils"
)

// Result holds the output of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// makeCommand creates a validated exec.Cmd using the sovereign utils.SafeCommand.
func makeCommand(ctx context.Context, command string, args []string) (*exec.Cmd, error) {
	return utils.SafeCommand(ctx, command, args...)
}

// Execute runs a command, capturing stdout and stderr concurrently via goroutines.
func Execute(ctx context.Context, command string, args []string) (*Result, error) {
	start := time.Now()

	cmd, err := makeCommand(ctx, command, args)
	if err != nil {
		return nil, fmt.Errorf("safe command: %w", err)
	}
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

	err = cmd.Wait()
	wg.Wait()

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
	cmd, err := makeCommand(ctx, command, args)
	if err != nil {
		return 1, fmt.Errorf("safe command: %w", err)
	}

	cmd.Stdin = os.Stdin
	// When running as MCP server, we MUST NOT write to os.Stdout directly
	// as it will corrupt the JSON-RPC stream. Redirecting to Stderr is safer.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("passthrough: %w", err)
	}
	return 0, nil
}
