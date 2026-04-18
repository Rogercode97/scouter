package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseWithTreeSitter_Calls(t *testing.T) {
	// Create a sample TS file
	content := []byte(`
		function caller() {
			callee();
			obj.method();
		}
		function callee() {}
	`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(filePath, content, 0644)

	ctx := context.Background()
	pointers, calls, err := ParseWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("ParseWithTreeSitter failed: %v", err)
	}

	if len(calls) == 0 {
		t.Errorf("Expected calls, got 0")
	}

	foundCallee := false
	for _, call := range calls {
		if call.CalleeName == "callee" || call.CalleeName == "method" {
			foundCallee = true
		}
	}
	if !foundCallee {
		t.Errorf("Did not find expected calls")
	}

	if len(pointers) == 0 {
		t.Errorf("Expected pointers, got 0")
	}
}
