package rtk

import (
	"context"
	"os/exec"
)

type Reader struct{}

func NewReader() *Reader {
	return &Reader{}
}

func (r *Reader) Read(ctx context.Context, path, pointer string) (string, bool, error) {
	if _, err := exec.LookPath("rtk"); err == nil {
		cmd := exec.CommandContext(ctx, "rtk", "read", path, "--pointer", pointer, "--ultra-compact")
		if out, err := cmd.CombinedOutput(); err == nil {
			return string(out), true, nil
		}
		// If it fails, we fall back instead of returning error
	}
	return "", false, nil
}
