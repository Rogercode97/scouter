package mcp

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func formatError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func formatResult(thought string, data any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	text := string(jsonData)
	if thought != "" {
		text = "<thought>\n" + thought + "\n</thought>\n" + text
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil
}

func formatTextResult(thought, text string) *mcp.CallToolResult {
	if thought != "" {
		text = "<thought>\n" + thought + "\n</thought>\n" + text
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
