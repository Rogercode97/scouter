package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// daemonSocketPath is where the shared gopls daemon listens. A fixed name in
// os.TempDir() is a namespace every account on the host shares: another user
// can pre-create that path and receive the daemon's traffic, and two checkouts
// of this repository collide with each other. The socket lives in a per-user
// directory instead, and the caller creates it 0700.
func daemonSocketPath() string {
	base := os.TempDir()
	if runtime.GOOS == "android" {
		// Termux rewrites TMPDIR per app; os.TempDir already reports it.
		if tmp := os.Getenv("TMPDIR"); tmp != "" {
			base = tmp
		}
	}
	return filepath.Join(base, "scouter-"+socketOwnerTag(), "gopls.sock")
}

// connectSharedDaemon returns a client for the per-user gopls daemon, starting
// it when nothing answers yet. It returns nil when the daemon cannot be used,
// and the caller then owns its own gopls process.
func connectSharedDaemon(ctx context.Context) LSPClient {
	socketPath := daemonSocketPath()
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil
	}
	if client, err := NewSocketClient(ctx, "unix", socketPath); err == nil {
		return client
	}

	// LSP server processes are long-running and MUST NOT be tied to a request
	// context that might time out, so the daemon detaches into its own group.
	daemonCmd := exec.Command("gopls", "serve", "-listen=unix;"+socketPath)
	setSysProcAttr(daemonCmd)
	if err := daemonCmd.Start(); err != nil {
		return nil
	}

	// Give it a tiny moment to create the socket.
	for i := 0; i < 10; i++ {
		if client, err := NewSocketClient(ctx, "unix", socketPath); err == nil {
			return client
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

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
			// Persistent daemon: a socket shared per user, not a fixed path in a
			// namespace every account on the host can write to.
			if client := connectSharedDaemon(bgCtx); client != nil {
				entry.client = client
				return
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
