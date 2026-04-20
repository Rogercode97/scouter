package lsp

import (
	"fmt"
	"path/filepath"
	"sync"
)

type clientEntry struct {
	client LSPClient
	err    error
	once   sync.Once
}

type Manager struct {
	clients map[string]*clientEntry
	mu      sync.RWMutex

	// clientCreator allows mocking for tests
	clientCreator func(binary string, args ...string) (LSPClient, error)
}

func NewManager() *Manager {
	return &Manager{
		clients:       make(map[string]*clientEntry),
		clientCreator: NewClient,
	}
}

func (m *Manager) GetClient(filePath string) (LSPClient, error) {
	ext := filepath.Ext(filePath)
	binary := ""
	var args []string

	switch ext {
	case ".go":
		binary = "gopls"
	case ".ts", ".tsx", ".js", ".jsx":
		binary = "typescript-language-server"
		args = []string{"--stdio"}
	default:
		return nil, fmt.Errorf("no LSP server configured for extension %s", ext)
	}

	m.mu.RLock()
	entry, ok := m.clients[ext]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		entry, ok = m.clients[ext]
		if !ok {
			entry = &clientEntry{}
			m.clients[ext] = entry
		}
		m.mu.Unlock()
	}

	entry.once.Do(func() {
		entry.client, entry.err = m.clientCreator(binary, args...)
	})

	if entry.err != nil {
		return nil, fmt.Errorf("failed to start LSP server %s: %w", binary, entry.err)
	}

	return entry.client, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range m.clients {
		if entry.client != nil {
			entry.client.Close()
		}
	}
	m.clients = make(map[string]*clientEntry)
	return nil
}
