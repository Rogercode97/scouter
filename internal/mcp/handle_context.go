package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)



type PureSignalParams struct {
	Text  string `json:"text" jsonschema:"REQUIRED. The raw text to filter and extract pure signal from"`
	Mode  string `json:"mode,omitempty" jsonschema:"Optional: Filtering mode (e.g., 'compact', 'verbose')"`
	Level string `json:"level,omitempty" jsonschema:"Optional: Filtering aggressiveness (e.g., 'aggressive', 'balanced')"`
}

type SaveAnchorParams struct {
	Summary string `json:"summary" jsonschema:"REQUIRED. The technical summary of the session to anchor in Engram"`
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

func (s *Server) handleSaveAnchor(ctx context.Context, req *mcp.CallToolRequest, args SaveAnchorParams) (*mcp.CallToolResult, any, error) {
	if args.Summary == "" {
		return nil, nil, fmt.Errorf("missing summary")
	}

	// [Singularity Upgrade] Invisible Orchestration
	project := utils.GetRepoName(ctx)
	if project == "" {
		project = "scouter-anchor"
	}
	now := time.Now().Format(time.RFC3339)
	title := fmt.Sprintf("[ANCHOR] Session State %s", now)
	engramContent := fmt.Sprintf("**What**: Latent session state compaction.\n**Why**: Context window optimization.\n**Where**: Project: %s\n**Learned**: %s", project, args.Summary)

	// [Sinergia Upgrade] Direct SQLite Persistence
	err := s.memory.SaveObservation(ctx, project, memory.DistilledMemory{
		Type:    "session_summary",
		Title:   title,
		Content: engramContent,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save anchor: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Session state anchored successfully in Engram: %s", title)},
		},
	}, nil, nil
}