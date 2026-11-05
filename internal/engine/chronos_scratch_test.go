package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScratchFileNeverReusesTheTargetName pins the property Chronos needs: the
// scratch copy it makes of a historical file must be a name that cannot already
// exist in the repository. The previous name was "<file>.scouter.tmp", which is
// a legal tracked path: the run overwrote it and then removed it.
func TestScratchFileNeverReusesTheTargetName(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "notes.go")
	squatter := target + ".scouter.tmp"
	const squatterContent = "a file the repository already contains"
	if err := os.WriteFile(squatter, []byte(squatterContent), 0o644); err != nil {
		t.Fatalf("seed squatter: %v", err)
	}

	first, err := writeScratchFile(dir, ".go", []byte("package notes\n"))
	if err != nil {
		t.Fatalf("scratch: %v", err)
	}
	defer os.Remove(first)

	if first == squatter {
		t.Fatalf("scratch file reused the predictable name %s", squatter)
	}
	if !strings.HasPrefix(filepath.Base(first), ".scouter-chrono-") {
		t.Fatalf("scratch name %q does not announce itself as scratch", filepath.Base(first))
	}
	if ext := filepath.Ext(first); ext != ".go" {
		t.Fatalf("scratch extension = %q, want .go: the parser is chosen by it", ext)
	}
	written, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read scratch: %v", err)
	}
	if string(written) != "package notes\n" {
		t.Fatalf("scratch content = %q", written)
	}

	if kept, err := os.ReadFile(squatter); err != nil || string(kept) != squatterContent {
		t.Fatalf("pre-existing %s was modified: %q err=%v", filepath.Base(squatter), kept, err)
	}

	second, err := writeScratchFile(dir, ".go", []byte("package notes\n"))
	if err != nil {
		t.Fatalf("second scratch: %v", err)
	}
	defer os.Remove(second)
	if first == second {
		t.Fatalf("two scratch files share the name %s", first)
	}
}
