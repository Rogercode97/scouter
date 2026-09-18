package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// storeLockTimeout bounds how long a process waits for the database lock.
// A stuck holder produces a clear actionable error instead of silently hanging.
// Configured as a variable so test suites can shorten the timeout path.
var storeLockTimeout = 30 * time.Second

// AcquireStoreLock takes an exclusive advisory lock on the database lock file
// associated with dbPath and returns a release function.
// If dbPath is an in-memory database (contains ":memory:"), locking is a no-op.
func AcquireStoreLock(dbPath string) (func(), error) {
	if strings.Contains(dbPath, ":memory:") {
		return func() {}, nil
	}

	cleanPath := filepath.Clean(dbPath)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("create directory for lock file %s: %w", cleanPath, err)
	}

	lockPath := cleanPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}

	deadline := time.Now().Add(storeLockTimeout)
	backoff := 10 * time.Millisecond

	for {
		acquired, err := tryLockFile(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("flock on %s: %w", lockPath, err)
		}
		if acquired {
			return func() {
				_ = unlockFile(f)
				_ = f.Close()
			}, nil
		}

		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf(
				"timed out after %s waiting for store lock %s — another scouter process appears to be holding it; check for running scouter processes before retrying",
				storeLockTimeout, lockPath,
			)
		}

		time.Sleep(backoff)
		if backoff < 100*time.Millisecond {
			backoff *= 2
			if backoff > 100*time.Millisecond {
				backoff = 100 * time.Millisecond
			}
		}
	}
}
