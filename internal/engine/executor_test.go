package engine

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecuteEcho(t *testing.T) {
	ctx := context.Background()
	result, err := Execute(ctx, "echo", []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello world" {
		t.Errorf("stdout = %q", got)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestExecuteStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "sh", []string{"-c", "echo error >&2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(result.Stderr); got != "error" {
		t.Errorf("stderr = %q", got)
	}
}

func TestExecuteExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "sh", []string{"-c", "exit 42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestExecuteNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := Execute(ctx, "nonexistent-command-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestPassthrough(t *testing.T) {
	ctx := context.Background()
	code, err := Passthrough(ctx, "echo", []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d", code)
	}
}

func TestPassthroughExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	code, err := Passthrough(ctx, "sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
}

func TestExecuteShellBuiltin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "export", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error executing shell builtin: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestPassthroughShellBuiltin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	code, err := Passthrough(ctx, "export", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestMakeCommandBuiltin(t *testing.T) {
	ctx := context.Background()
	cmd := makeCommand(ctx, "export", []string{"A=1", "B=2"})
	if cmd.Path == "" {
		t.Fatal("command path should not be empty")
	}
	// Should wrap with sh -c
	if cmd.Args[0] != "sh" {
		t.Errorf("expected sh wrapper, got %q", cmd.Args[0])
	}
	if cmd.Args[1] != "-c" {
		t.Errorf("expected -c flag, got %q", cmd.Args[1])
	}
}

func TestMakeCommandRegular(t *testing.T) {
	ctx := context.Background()
	cmd := makeCommand(ctx, "git", []string{"status"})
	// Should NOT wrap with sh
	if len(cmd.Args) > 0 && cmd.Args[0] == "sh" {
		t.Error("regular commands should not be wrapped with sh")
	}
}

// TestExecuteCapturesEveryRun pins the os/exec contract for StdoutPipe: Wait
// must not run before the readers drained the pipes, or output is silently
// lost. The window is a scheduling race, not a size one — with a payload
// larger than the pipe buffer the child blocks, so the readers never fall
// behind and the bug hides. Small output plus repetition is what exposes it;
// under -race with GOMAXPROCS=1 this failed 150 of 150 runs before the fix.
func TestExecuteCapturesEveryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()

	const runs = 200
	for i := 0; i < runs; i++ {
		result, err := Execute(ctx, "echo", []string{"hello", "world"})
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		if got := strings.TrimSpace(result.Stdout); got != "hello world" {
			t.Fatalf("run %d: stdout = %q, want %q", i, got, "hello world")
		}

		result, err = Execute(ctx, "sh", []string{"-c", "echo error >&2"})
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		if got := strings.TrimSpace(result.Stderr); got != "error" {
			t.Fatalf("run %d: stderr = %q, want %q", i, got, "error")
		}
	}
}

// TestExecuteSuccessfulCommandDrainsInheritedOutput pins the property the
// cancellation bound must not break: a command that exits successfully is never
// cut off, even when a grandchild it spawned still holds the pipe and writes
// after the parent is gone.
func TestExecuteSuccessfulCommandDrainsInheritedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	original := drainGrace
	drainGrace = 50 * time.Millisecond
	t.Cleanup(func() { drainGrace = original })

	result, err := Execute(context.Background(), "sh",
		[]string{"-c", "echo first; ( sleep 0.3; echo second ) &"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(result.Stdout)
	if !strings.Contains(got, "first") {
		t.Fatalf("lost the command's own output: %q", got)
	}
	if !strings.Contains(got, "second") {
		t.Fatalf("a successful command must drain its inherited pipe, got %q", got)
	}
}

// TestExecuteCancellationIsBounded is a liveness guard, so elapsed time is the
// observable: with a grandchild holding the pipe, cancellation must still return
// after the grace instead of waiting for the holder to finish.
func TestExecuteCancellationIsBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	original := drainGrace
	drainGrace = 100 * time.Millisecond
	t.Cleanup(func() { drainGrace = original })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	const holderLifetime = 10 * time.Second
	start := time.Now()
	result, err := Execute(ctx, "sh", []string{"-c", "sleep " + "10" + " & wait"})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("cancellation must not surface as an error here: %v", err)
	}
	if result == nil {
		t.Fatal("expected a result")
	}
	if elapsed > holderLifetime/2 {
		t.Fatalf("cancellation waited %v, which is not bounded by the grace", elapsed)
	}
}
