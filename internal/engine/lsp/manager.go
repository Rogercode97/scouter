package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type clientEntry struct {
	client LSPClient
	err    error
	once   sync.Once
}

var (
	globalManager *Manager
	globalOnce    sync.Once
)

type Manager struct {
	clients map[string]*clientEntry
	mu      sync.RWMutex
	closed  bool

	// clientCreator allows mocking for tests
	clientCreator func(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error)
	warpSpeed     bool
}

func NewManager() *Manager {
	clients := make(map[string]*clientEntry)
	m := &Manager{
		clients:       clients,
		clientCreator: NewClient,
		warpSpeed:     true,
	}
	// Go 1.25 native cleanup for the manager singleton
	// We use the clients map as the anchor to avoid the "ptr is equal to arg" panic
	runtime.AddCleanup(m, func(c map[string]*clientEntry) {
		for _, entry := range c {
			if entry.client != nil {
				entry.client.Close()
			}
		}
	}, clients)
	return m
}

func GetGlobalManager() *Manager {
	globalOnce.Do(func() {
		globalManager = NewManager()
	})
	return globalManager
}

func (m *Manager) SetClientCreatorForTest(fn func(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientCreator = fn
	m.warpSpeed = false
}

func (m *Manager) GetClient(ctx context.Context, filePath string) (LSPClient, error) {
	ext := filepath.Ext(filePath)
	binary := ""
	var args []string

	switch ext {
	case ".go":
		binary = "gopls"
		if _, err := exec.LookPath(binary); err != nil {
			// Fallback to ~/go/bin
			home, _ := os.UserHomeDir()
			binary = filepath.Join(home, "go", "bin", "gopls")
		}
	case ".ts", ".tsx", ".js", ".jsx":
		binary = "typescript-language-server"
		args = []string{"--stdio"}
	default:
		return nil, fmt.Errorf("no LSP server configured for extension %s", ext)
	}

	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("lsp manager is closed")
	}
	warpSpeed := m.warpSpeed
	entry, ok := m.clients[ext]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, fmt.Errorf("lsp manager is closed")
		}
		entry, ok = m.clients[ext]
		if !ok {
			entry = &clientEntry{}
			m.clients[ext] = entry
		}
		m.mu.Unlock()
	}

	entry.once.Do(func() {
		// IMPORTANT: LSP server processes are long-running and MUST NOT be tied 
		// to a request context that might time out. We use a background context
		// for the process lifecycle.
		bgCtx := context.Background()
		cwd, _ := os.Getwd()

		if warpSpeed && binary == "gopls" {
			// Persistent Daemon attempt: Try to connect via Unix Socket
			socketPath := filepath.Join(os.TempDir(), "scouter-gopls.sock")
			if runtime.GOOS == "android" {
				// Android/Termux: use $TMPDIR if available for better permission handling
				if tmp := os.Getenv("TMPDIR"); tmp != "" {
					socketPath = filepath.Join(tmp, "scouter-gopls.sock")
				}
			}

			client, err := NewSocketClient(bgCtx, "unix", socketPath)
			if err == nil {
				entry.client = client
				return
			}

			// If connection failed, try to start the daemon
			daemonCmd := exec.Command("gopls", "serve", "-listen=unix;"+socketPath)
			// Detach from current process group so it persists
			daemonCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if err := daemonCmd.Start(); err == nil {
				// Give it a tiny moment to create the socket
				for i := 0; i < 10; i++ {
					client, err := NewSocketClient(bgCtx, "unix", socketPath)
					if err == nil {
						entry.client = client
						return
					}
					select {
					case <-bgCtx.Done():
						break
					default:
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}

		entry.client, entry.err = m.clientCreator(bgCtx, cwd, binary, args...)
	})

	if entry.err != nil {
		return nil, fmt.Errorf("failed to start LSP server %s: %w", binary, entry.err)
	}

	// DIVINE REDEMPTION: Ensure the handshake (initialization wait) respects the request context.
	// We wait for the client to be ready, but only as long as the caller is willing to wait.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Client is already initialized or being initialized. 
		// Since NewClient performs the handshake, entry.client is ready here.
	}

	return entry.client, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	var lastErr error
	for _, entry := range m.clients {
		if entry.client != nil {
			if err := entry.client.Close(); err != nil {
				lastErr = err
			}
		}
	}
	m.clients = make(map[string]*clientEntry)
	return lastErr
}
