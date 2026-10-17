package utils

import (
	"context"
	"path/filepath"
	"testing"
)

// TestSafeCommandRejectsExecutionBypasses covers S1 (the two total bypasses),
// S2 (the three production call sites keep working) and S5 (pre-existing
// rejections must not regress).
func TestSafeCommandRejectsExecutionBypasses(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		binary  string
		args    []string
		wantErr bool
	}{
		{"shell -c payload", "sh", []string{"-c", "echo pwned"}, true},
		{"bash -c payload", "bash", []string{"-c", "echo pwned"}, true},
		{"eval payload", "eval", []string{"echo pwned"}, true},
		{"source payload", "source", []string{"/tmp/payload.sh"}, true},
		{"forbidden binary", "rm", []string{"-rf", "/"}, true},
		{"denylisted character in argument", "go", []string{"test", "./...; rm -rf /"}, true},
		{"caller rtk", "rtk", []string{"read", "a.go", "--pointer", "p", "--ultra-compact"}, false},
		{"caller sg", "sg", []string{"run", "--pattern", "p", "--rewrite", "r", "a.go"}, false},
		{"caller git", "git", []string{"diff", "HEAD", "--unified=0"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := SafeCommand(ctx, tt.binary, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SafeCommand(%q, %v) error = %v, wantErr %v", tt.binary, tt.args, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cmd == nil {
				t.Fatalf("SafeCommand(%q, %v) returned a nil command without error", tt.binary, tt.args)
			}
			if base := filepath.Base(cmd.Path); base == "sh" || base == "bash" {
				t.Fatalf("SafeCommand(%q, %v) resolved to shell %q", tt.binary, tt.args, base)
			}
		})
	}
}

// FuzzSafeCommand asserts the S1 invariant: whatever the caller asks for, a
// command that is returned must come from the allow-list and must never be a
// shell. The seed corpus contains the known bypass tuples.
func FuzzSafeCommand(f *testing.F) {
	f.Add("sh", "-c")
	f.Add("bash", "-c")
	f.Add("eval", "echo pwned")
	f.Add("source", "/tmp/payload.sh")
	f.Add("git", "diff")
	f.Add("go", "./...; rm -rf /")

	allowed := map[string]bool{
		"git":      true,
		"go":       true,
		"rtk":      true,
		"sg":       true,
		"scouter":  true,
		"gopls":    true,
		"ast-grep": true,
	}
	shells := map[string]bool{
		"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
	}

	f.Fuzz(func(t *testing.T, name string, arg string) {
		if len(name) > 256 || len(arg) > 4096 {
			return
		}
		cmd, err := SafeCommand(context.Background(), name, arg)
		if err != nil {
			return
		}
		if cmd == nil {
			t.Fatalf("SafeCommand(%q, %q) returned a nil command without error", name, arg)
		}
		base := filepath.Base(cmd.Path)
		if shells[base] {
			t.Fatalf("SafeCommand(%q, %q) resolved to shell %q", name, arg, base)
		}
		if !allowed[base] {
			t.Fatalf("SafeCommand(%q, %q) accepted a binary outside the allow-list: %q", name, arg, base)
		}
	})
}

// TestSafeCommandRejectsInjectedFlags covers S7 (CWE-88, the previously open
// S6 gap). A flag the binary does not need can turn an allow-listed program
// into a code-loading vector: `git diff --upload-pack=/bin/sh` runs a shell.
// The policy is that a token starting with "-" is either an allowed flag for
// that binary or it is rejected, so the flags the three production callers
// actually pass keep working and nothing else gets through.
func TestSafeCommandRejectsInjectedFlags(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		binary  string
		args    []string
		wantErr bool
	}{
		{"git upload-pack, the S6 example", "git", []string{"diff", "HEAD", "--upload-pack=/bin/sh"}, true},
		{"git output redirected to a file", "git", []string{"diff", "HEAD", "--output=/tmp/leak"}, true},
		{"git short flag", "git", []string{"-c", "core.pager=sh"}, true},
		{"rtk unknown flag", "rtk", []string{"read", "a.go", "--exec=sh"}, true},
		{"ast-grep unknown flag", "ast-grep", []string{"run", "--pattern", "p", "--stdin"}, true},
		{"go has no allowed flags", "go", []string{"test", "./...", "--exec=/bin/sh"}, true},
		{"flag-shaped value after an allowed flag", "rtk", []string{"read", "a.go", "--pointer", "--upload-pack=/bin/sh"}, true},
		{"caller git still works", "git", []string{"diff", "HEAD", "--unified=0"}, false},
		{"caller rtk still works", "rtk", []string{"read", "a.go", "--pointer", "p", "--ultra-compact"}, false},
		{"caller sg still works", "sg", []string{"run", "--pattern", "p", "--rewrite", "r", "a.go"}, false},
		{"caller ast-grep still works", "ast-grep", []string{"run", "--pattern", "p", "--rewrite", "r", "a.go"}, false},
		{"allowed flag with attached value", "git", []string{"diff", "HEAD", "--unified=3"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := SafeCommand(ctx, tt.binary, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SafeCommand(%q, %v) error = %v, wantErr %v", tt.binary, tt.args, err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				return
			}
			if cmd == nil {
				t.Fatalf("SafeCommand(%q, %v) returned a nil command without error", tt.binary, tt.args)
			}
		})
	}
}
