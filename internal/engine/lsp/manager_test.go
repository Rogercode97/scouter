package lsp

import (
	"context"
	"testing"
)

type mockClient struct {
	binary string
}

func (c *mockClient) Definition(ctx context.Context, params DefinitionParams) ([]Location, error) {
	return nil, nil
}
func (c *mockClient) Hover(ctx context.Context, params HoverParams) (*Hover, error) {
	return nil, nil
}
func (c *mockClient) Close() error {
	return nil
}

func TestLSPManager(t *testing.T) {
	m := NewManager()

	// Mock client creation
	m.clientCreator = func(binary string, args ...string) (LSPClient, error) {
		return &mockClient{binary: binary}, nil
	}

	// Test extension mapping
	ext := "test.go"
	client, err := m.GetClient(ext)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	mc := client.(*mockClient)
	if mc.binary != "gopls" {
		t.Errorf("Expected binary gopls, got %s", mc.binary)
	}

	// Test caching
	client2, _ := m.GetClient(ext)
	if client != client2 {
		t.Errorf("Client not cached")
	}
}
