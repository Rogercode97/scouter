package mcp

import (
	"context"
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
		MaxTokens:    1024,
	})
	if err != nil {
		return "", err
	}

	if txt, ok := res.Content.(*mcp.TextContent); ok {
		return txt.Text, nil
	}

	return "", nil
}
