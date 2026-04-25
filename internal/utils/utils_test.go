package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_Security(t *testing.T) {
	tmp := os.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errSub  string
	}{
		{
			name:    "Valid relative path",
			path:    "go.mod",
			wantErr: false,
		},
		{
			name:    "Path traversal attempt",
			path:    "../../../../etc/passwd",
			wantErr: true,
			errSub:  "escapes sovereignty",
		},
		{
			name:    "Blacklist: .git",
			path:    ".git/config",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Blacklist: .env",
			path:    ".env",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Blacklist Case-Insensitivity: .GIT",
			path:    ".GIT/config",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Valid temp path",
			path:    filepath.Join(tmp, "scouter-test.txt"),
			wantErr: false,
		},
		{
			name:    "Absolute path violation",
			path:    "/etc/passwd",
			wantErr: true,
			errSub:  "absolute paths outside /tmp are prohibited",
		},
		{
			name:    "Project inside restricted folder (Parent Pollution Fix)",
			path:    "src/main.go", 
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("ValidatePath() error = %v, wantErrSub %s", err, tt.errSub)
			}
		})
	}
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	root, _ := GetRepoRoot()

	// Create a symlink inside the project pointing to a forbidden directory
	evilLink := filepath.Join(root, "evil_link")
	os.Remove(evilLink)
	
	forbiddenDir := filepath.Dir(root) 
	err := os.Symlink(forbiddenDir, evilLink)
	if err != nil {
		t.Skip("Skipping symlink test (permissions or OS limitation)")
		return
	}
	defer os.Remove(evilLink)

	// Attempt to access a file THROUGH the symlink that doesn't exist yet
	pathThroughLink := filepath.Join("evil_link", "new_secret.txt")
	_, err = ValidatePath(pathThroughLink)
	if err == nil {
		t.Error("expected error for path escaping through symlink")
	} else if !strings.Contains(err.Error(), "security violation") {
		t.Errorf("expected security violation, got: %v", err)
	}
}

func TestGetRepoRoot(t *testing.T) {
	root, err := GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot failed: %v", err)
	}
	if !strings.Contains(root, "scouter") {
		t.Errorf("expected root to contain 'scouter', got %s", root)
	}
}
