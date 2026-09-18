package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireStoreLock_Memory(t *testing.T) {
	unlock, err := AcquireStoreLock(":memory:")
	require.NoError(t, err)
	require.NotNil(t, unlock)
	assert.NotPanics(t, func() {
		unlock()
	})
}

func TestAcquireStoreLock_ExclusiveAndRelease(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	unlock1, err := AcquireStoreLock(dbPath)
	require.NoError(t, err)
	require.NotNil(t, unlock1)

	// Save original timeout and restore it
	origTimeout := storeLockTimeout
	storeLockTimeout = 50 * time.Millisecond
	defer func() { storeLockTimeout = origTimeout }()

	// Second acquisition should time out while unlock1 is held
	start := time.Now()
	_, err2 := AcquireStoreLock(dbPath)
	elapsed := time.Since(start)

	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "timed out")
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)

	// Release first lock
	unlock1()

	// Third acquisition should now succeed immediately
	unlock3, err3 := AcquireStoreLock(dbPath)
	require.NoError(t, err3)
	require.NotNil(t, unlock3)
	unlock3()

	// Verify lock file remains in place (no inode deletion race)
	_, errStat := os.Stat(dbPath + ".lock")
	assert.NoError(t, errStat)
}

func TestNewStore_ConcurrentInitialization(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent.db")

	const workers = 3
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			s, err := NewStore(t.Context(), dbPath)
			if err != nil {
				errCh <- err
				return
			}
			// Run a read query to ensure DB is healthy
			_, _, err = s.GetStats(t.Context())
			errCh <- err
		}()
	}

	for i := 0; i < workers; i++ {
		err := <-errCh
		require.NoError(t, err)
	}
}
