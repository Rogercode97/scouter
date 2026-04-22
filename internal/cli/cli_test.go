package cli

import (
	"context"
	"io"
	"testing"
)

func TestRunRejectsCd(t *testing.T) {
	ctx := context.Background()
	// Run now takes ctx, args, stdout, stderr
	code := Run(ctx, []string{"scouter", "cd", "/tmp"}, io.Discard, io.Discard)
	// If cd is not found by exec, Passthrough will likely return an error and Run might return 1
	if code == 0 {
		t.Errorf("Run(cd) should fail, got %d", code)
	}
}
