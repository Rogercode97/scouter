package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PureSignalParams struct {
	Text  string `json:"text" jsonschema:"REQUIRED. The raw text to filter and extract pure signal from"`
	Mode  string `json:"mode,omitempty" jsonschema:"Optional: Filtering mode (e.g., 'compact', 'verbose')"`
	Level string `json:"level,omitempty" jsonschema:"Optional: Filtering aggressiveness (e.g., 'aggressive', 'balanced')"`
}

func (s *Server) handlePureSignal(ctx context.Context, req *mcp.CallToolRequest, args PureSignalParams) (*mcp.CallToolResult, any, error) {
	if args.Text == "" {
		return nil, nil, fmt.Errorf("missing 'text' argument")
	}

	level := args.Level
	if level == "" {
		level = "aggressive"
	}

	fn, ok := filter.GetAction("pure_signal")
	if !ok {
		return nil, nil, fmt.Errorf("pure_signal action not found")
	}

	res, err := fn(ctx, filter.ActionResult{Lines: strings.Split(args.Text, "\n"), Metadata: make(map[string]any)}, map[string]any{"level": level})
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(res.Lines, "\n")},
		},
	}, nil, nil
}
