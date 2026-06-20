package rtk

import (
	"context"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/utils"
)

type Reader struct{}

func NewReader() *Reader {
	return &Reader{}
}

func (r *Reader) Read(ctx context.Context, path, pointer string) (string, bool, error) {
	if _, err := exec.LookPath("rtk"); err == nil {
		cmd, err := utils.SafeCommand(ctx, "rtk", "read", path, "--pointer", pointer, "--ultra-compact")
		if err == nil {
			if out, cmdErr := cmd.CombinedOutput(); cmdErr == nil {
				return string(out), true, nil
			}
		}
		// If it fails, we fall back instead of returning error
	}
	return "", false, nil
}
