//go:build windows

package lsp

import (
	"os"
	"os/exec"
	"strings"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// No Setpgid for Windows
}

// socketOwnerTag separates the per-user daemon directory. Windows has no uid,
// so the account name stands in for it with everything a path cannot carry
// removed.
func socketOwnerTag() string {
	user := strings.TrimSpace(os.Getenv("USERNAME"))
	user = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', ' ':
			return -1
		}
		return r
	}, user)
	if user == "" {
		return "user"
	}
	return user
}
