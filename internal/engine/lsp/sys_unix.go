//go:build !windows

package lsp

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// socketOwnerTag separates the per-user daemon directory, so two accounts on
// the same host never share one socket.
func socketOwnerTag() string {
	return strconv.Itoa(os.Getuid())
}
