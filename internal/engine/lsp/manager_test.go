package lsp

import (
	"context"
	"strings"
	"testing"
)

type mockClient struct {
	binary string
}

func (c *mockClient) Definition(ctx context.Context, params DefinitionParams) ([]Location, error) {
	return nil, nil
}
func (c *mockClient) Implementation(ctx context.Context, params ImplementationParams) ([]Location, error) {
	return nil, nil
}
func (c *mockClient) References(ctx context.Context, params ReferenceParams) ([]Location, error) {
	return nil, nil
}
func (c *mockClient) Hover(ctx context.Context, params HoverParams) (*Hover, error) {
	return nil, nil
}
func (c *mockClient) PrepareCallHierarchy(ctx context.Context, params CallHierarchyPrepareParams) ([]CallHierarchyItem, error) {
	return nil, nil
}
func (c *mockClient) IncomingCalls(ctx context.Context, params CallHierarchyIncomingCallsParams) ([]CallHierarchyIncomingCall, error) {
	return nil, nil
}
func (c *mockClient) OutgoingCalls(ctx context.Context, params CallHierarchyOutgoingCallsParams) ([]CallHierarchyOutgoingCall, error) {
	return nil, nil
}
func (c *mockClient) Close() error {
	return nil
}

func TestLSPManager(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	// Mock client creation
	m.clientCreator = func(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error) {
		return &mockClient{binary: binary}, nil
	}

	// Test extension mapping
	ext := "test.go"
	client, err := m.GetClient(ctx, ext)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	mc := client.(*mockClient)
	if !strings.HasSuffix(mc.binary, "gopls") {
		t.Errorf("Expected binary to end with gopls, got %s", mc.binary)
	}

	// Test caching
	client2, _ := m.GetClient(ctx, ext)
	if client != client2 {
		t.Errorf("Client not cached")
	}
}
