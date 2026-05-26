package mcp

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPMessenger adapts the MCP Sampling protocol to the engine.Messenger interface.
type MCPMessenger struct {
	session *mcp.ServerSession
}

// NewMCPMessenger creates a new messenger for the given MCP session.
func NewMCPMessenger(session *mcp.ServerSession) *MCPMessenger {
	return &MCPMessenger{session: session}
}

// Ask sends a sampling request to the MCP client.
func (m *MCPMessenger) Ask(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	initParams := m.session.InitializeParams()
	if initParams == nil || initParams.Capabilities == nil || initParams.Capabilities.Sampling == nil {
		return "", fmt.Errorf("client does not support sampling capabilities")
	}

	res, err := m.session.CreateMessage(ctx, &mcp.CreateMessageParams{
		Messages: []*mcp.SamplingMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: userPrompt,
				},
			},
		},
		SystemPrompt: systemPrompt,
		MaxTokens:    4096, // High budget for code transformations
	})
	if err != nil {
		return "", fmt.Errorf("sampling request failed: %w", err)
	}

	// Handle standard text response
	if txt, ok := res.Content.(*mcp.TextContent); ok {
		return txt.Text, nil
	}

	// Fallback for complex content or empty response
	return "", fmt.Errorf("unexpected content type in sampling response: %T", res.Content)
}
