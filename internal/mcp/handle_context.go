package mcp

import (
	"context"
	"fmt"
	"os/exec"
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

type CompactContextParams struct {
	Force bool `json:"force,omitempty" jsonschema:"Optional: Force compaction even if window limits aren't reached"`
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



func (s *Server) handleCompactContext(ctx context.Context, req *mcp.CallToolRequest, args CompactContextParams) (*mcp.CallToolResult, any, error) {
	// [Strike 5] Predictive Context: Identify critical hotspots for high-fidelity summary
	systemPrompt := CompactContextSystemPrompt
	diffOut, err := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0").Output()
	if err == nil && len(diffOut) > 0 {
		diff := string(diffOut)
		critical, _ := s.engine.IdentifyCriticalContext(ctx, diff)
		if len(critical) > 0 {
			var sb strings.Builder
			sb.WriteString(systemPrompt)
			sb.WriteString("\n\nCRITICAL SYMBOLS INVOLVED:\n")
			for _, c := range critical {
				sb.WriteString(fmt.Sprintf("- %s (File: %s, Risk: %.2f)\n", c.Symbol, c.File, c.RiskScore))
			}
			sb.WriteString("Please ensure these are documented with high fidelity.")
			systemPrompt = sb.String()
		}
	}

	// Log user prompt (the request for compaction)
	s.AppendSessionMessage(memory.Message{Role: "user", Content: "Requesting context compaction."})

	// 1. Sampling Request (Self-Summarization Loop)
	samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: "Please provide the high-density technical summary for compaction."}},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sampling for compaction failed: %w", err)
	}

	txt, ok := samplingRes.Content.(*mcp.TextContent)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected sampling response type: %T", samplingRes.Content)
	}

	// Log assistant response
	s.AppendSessionMessage(memory.Message{Role: "assistant", Content: txt.Text})

	// 2. Delegate to TruthEngine for compaction
	res, err := s.engine.CompactSession(ctx, txt.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("compaction failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res.Message},
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
	err := s.engine.MemoryProvider().SaveObservation(ctx, project, memory.DistilledMemory{
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
