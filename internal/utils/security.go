package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath ensures the filePath is absolute, resolved (no symlink tricks),
// and stays within the allowed home directory.
func ValidatePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Resolve symlinks to prevent escaping via link tricks
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet, we still check the prefix on the absPath
		realPath = absPath
	}

	home, _ := os.UserHomeDir()
	tmp := os.TempDir()
	if !strings.HasPrefix(realPath, home) &&
		!strings.HasPrefix(realPath, "/data/data/com.termux/files/usr/tmp/") &&
		!strings.HasPrefix(realPath, tmp) {
		return "", fmt.Errorf("security violation: access denied to path %s", realPath)
	}

	return realPath, nil
}
