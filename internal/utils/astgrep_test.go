package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// stubBinaries writes executable stubs into a temp directory and points PATH at
// it, so the test decides which candidates exist instead of the host machine.
func stubBinaries(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("writing stub %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestAstGrepBinary(t *testing.T) {
	tests := []struct {
		name    string
		present []string
		want    string
		wantErr bool
	}{
		{name: "prefers sg when both are installed", present: []string{"sg", "ast-grep"}, want: "sg"},
		{name: "falls back to ast-grep when sg is absent", present: []string{"ast-grep"}, want: "ast-grep"},
		{name: "accepts sg alone", present: []string{"sg"}, want: "sg"},
		{name: "reports when neither is installed", present: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubBinaries(t, tt.present...)

			got, err := AstGrepBinary()
			if (err != nil) != tt.wantErr {
				t.Fatalf("AstGrepBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("AstGrepBinary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// AstGrepBinary returns the bare name, never a resolved absolute path:
// SafeCommand validates callers against an allow-list of names, so a path would
// be rejected there and every structural refactor would fail on the machines
// where only the long name is installed.
func TestAstGrepBinaryReturnsNameNotPath(t *testing.T) {
	stubBinaries(t, "ast-grep")

	got, err := AstGrepBinary()
	if err != nil {
		t.Fatalf("AstGrepBinary() error = %v", err)
	}
	if got != "ast-grep" || filepath.IsAbs(got) {
		t.Errorf("AstGrepBinary() = %q, want the bare name %q", got, "ast-grep")
	}
}
