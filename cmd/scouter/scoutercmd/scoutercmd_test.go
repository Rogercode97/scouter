package scoutercmd

import (
	"context"
	"os"
	"testing"
)

func TestRunRejectsCd(t *testing.T) {
	ctx := context.Background()
	
	// Create dummy files for stdout/stderr
	fOut, _ := os.CreateTemp("", "out")
	defer os.Remove(fOut.Name())
	fErr, _ := os.CreateTemp("", "err")
	defer os.Remove(fErr.Name())

	code := Execute(ctx, []string{"cd", "/tmp"}, fOut, fErr)
	if code == 0 {
		t.Errorf("Execute(cd) should fail, got %d", code)
	}
}
