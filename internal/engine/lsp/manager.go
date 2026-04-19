package lsp

import (
	"fmt"
	"path/filepath"
	"sync"
)

type Manager struct {
	clients map[string]LSPClient
	mu      sync.RWMutex
	
	// clientCreator allows mocking for tests
	clientCreator func(binary string, args ...string) (LSPClient, error)
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]LSPClient),
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
	client, ok := m.clients[ext]
	m.mu.RUnlock()
	
	if ok {
		return client, nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check after lock
	if client, ok := m.clients[ext]; ok {
		return client, nil
	}
	
	client, err := m.clientCreator(binary, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to start LSP server %s: %w", binary, err)
	}
	
	m.clients[ext] = client
	return client, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]LSPClient)
	return nil
}
