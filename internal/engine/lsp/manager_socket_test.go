package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDaemonSocketPathIsPerUser pins the namespace the shared gopls daemon
// listens in. A fixed "scouter-gopls.sock" inside os.TempDir() is shared by
// every account on the host: another user can bind that path first and read the
// traffic, and every checkout of this repository competes for it.
func TestDaemonSocketPathIsPerUser(t *testing.T) {
	path := daemonSocketPath()

	shared := filepath.Join(os.TempDir(), "scouter-gopls.sock")
	if path == shared {
		t.Fatalf("daemon socket still lives at the shared path %s", shared)
	}

	dir := filepath.Dir(path)
	wantDir := filepath.Join(os.TempDir(), "scouter-"+socketOwnerTag())
	if dir != wantDir {
		t.Fatalf("socket dir = %s, want %s", dir, wantDir)
	}
	if !strings.HasPrefix(filepath.Base(dir), "scouter-") {
		t.Fatalf("socket dir %s does not identify its owner", dir)
	}
	if tag := socketOwnerTag(); tag == "" {
		t.Fatal("socketOwnerTag is empty: every user would share one directory")
	}

	if again := daemonSocketPath(); again != path {
		t.Fatalf("socket path is not stable: %s then %s: a daemon could never be found twice", path, again)
	}
}
