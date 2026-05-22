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
