package display

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Presenter defines the interface for formatting tool responses.
type Presenter interface {
	FormatResult(thought string, data interface{}) (*mcp.CallToolResult, error)
	FormatTextResult(thought string, text string) *mcp.CallToolResult
	FormatError(err error) *mcp.CallToolResult
}

// DefaultPresenter implements the Presenter interface.
type DefaultPresenter struct{}

// NewDefaultPresenter creates a new DefaultPresenter.
func NewDefaultPresenter() *DefaultPresenter {
	return &DefaultPresenter{}
}

// FormatResult wraps the output data in a JSON string, prepended by the thought block.
func (p *DefaultPresenter) FormatResult(thought string, data interface{}) (*mcp.CallToolResult, error) {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("<thought>\n%s\n</thought>\n%s", thought, string(out))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}, nil
}

// FormatTextResult wraps the text output, prepended by the thought block.
func (p *DefaultPresenter) FormatTextResult(thought string, text string) *mcp.CallToolResult {
	fullText := fmt.Sprintf("<thought>\n%s\n</thought>\n%s", thought, text)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fullText,
			},
		},
	}
}

// FormatError creates an MCP error result.
func (p *DefaultPresenter) FormatError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: err.Error(),
			},
		},
		IsError: true,
	}
}
