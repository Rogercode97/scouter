package main

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("test", "1.0.0")
	
	// VIOLATION: No validation, uses context.Background(), raw output
	s.AddTool(mcp.NewTool("unsafe_tool"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		path := args["path"].(string)
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Path is " + path}},
		}, nil
	})
}
