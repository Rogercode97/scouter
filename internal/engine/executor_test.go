package engine

import (
	"runtime"
	"strings"
	"testing"
)

func TestExecuteEcho(t *testing.T) {
	result, err := Execute("echo", []string{"hello", "world"})
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
	result, err := Execute("sh", []string{"-c", "echo error >&2"})
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
	result, err := Execute("sh", []string{"-c", "exit 42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestExecuteNotFound(t *testing.T) {
	_, err := Execute("nonexistent-command-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestPassthrough(t *testing.T) {
	code, err := Passthrough("echo", []string{"test"})
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
	code, err := Passthrough("sh", []string{"-c", "exit 7"})
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
	result, err := Execute("export", []string{"FOO=bar"})
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
	code, err := Passthrough("export", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestMakeCommandBuiltin(t *testing.T) {
	cmd := makeCommand("export", []string{"A=1", "B=2"})
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
	cmd := makeCommand("git", []string{"status"})
	// Should NOT wrap with sh
	if len(cmd.Args) > 0 && cmd.Args[0] == "sh" {
		t.Error("regular commands should not be wrapped with sh")
	}
}
