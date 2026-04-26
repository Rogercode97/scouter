package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tmpDir := os.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid relative path", "go.mod", false},
		{"Valid nested path", "internal/utils/security.go", false},
		{"Valid temp path", filepath.Join(tmpDir, "test.txt"), false},
		{"Jailbreak attempt (parent)", "../../etc/passwd", true},
		{"Absolute path violation", "/etc/passwd", true},
		{"Empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFTS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Simple term", "scouter", "\"scouter\""},
		{"With wildcard", "scout*", "\"scout\"*"},
		{"Internal quote", "don't", "\"don't\""},
		{"Control characters", "OR AND NEAR", "\"OR AND NEAR\""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFTS(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeFTS(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeFTS_Manual(t *testing.T) {
	// Let's do some manual checks to see what the logic actually produces
	inputs := []string{"test", "test*", "*test", "don't", "\"quoted\""}
	for _, in := range inputs {
		t.Logf("Input: %q -> Got: %q", in, SanitizeFTS(in))
	}
}
