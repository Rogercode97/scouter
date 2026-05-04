package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for execCommand mocking.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "engram":
		if len(args) > 0 && args[0] == "search" {
			query := args[1]
			if query == "error" {
				fmt.Fprintln(os.Stderr, "simulated error")
				os.Exit(1)
			}
			if query == "long" {
				fmt.Print(strings.Repeat("a", 1500))
				os.Exit(0)
			}
			fmt.Printf("simulated result for %s", query)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "Unknown command\n")
	os.Exit(2)
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestFetchEngramContext(t *testing.T) {
	originalExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = originalExecCommand }()

	t.Run("successful search", func(t *testing.T) {
		res := fetchEngramContext("normal_query")
		expected := "simulated result for normal_query"
		if res != expected {
			t.Errorf("expected %q, got %q", expected, res)
		}
	})

	t.Run("command error", func(t *testing.T) {
		res := fetchEngramContext("error")
		if res != "" {
			t.Errorf("expected empty string on error, got %q", res)
		}
	})

	t.Run("truncates long output", func(t *testing.T) {
		res := fetchEngramContext("long")
		if len(res) <= 1000 {
			t.Errorf("expected length > 1000, got %d", len(res))
		}
		if !strings.HasSuffix(res, "\n...[truncated]") {
			t.Errorf("expected truncation suffix, got %q", res)
		}
		if len(res) != 1000+len("\n...[truncated]") {
			t.Errorf("expected exactly 1000 chars + suffix, got %d", len(res))
		}
	})
}