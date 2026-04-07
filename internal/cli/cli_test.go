package cli

import "testing"

func TestUnproxyableCommands(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"cd", true},
		{"source", true},
		{".", true},
		{"git", false},
		{"go", false},
		{"export", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := unproxyableReason(tt.command) != ""
			if got != tt.want {
				t.Errorf("unproxyableReason(%q) returned %q, wantBlocked=%v", tt.command, unproxyableReason(tt.command), tt.want)
			}
		})
	}
}

func TestRunRejectsCd(t *testing.T) {
	code := Run([]string{"snip", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(cd) = %d, want 1", code)
	}
}
