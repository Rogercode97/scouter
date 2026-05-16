package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HybridSearchParams struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type PureSignalParams struct {
	Text  string `json:"text"`
	Mode  string `json:"mode,omitempty"`
	Level string `json:"level,omitempty"`
}

type CompactContextParams struct {
	Force bool `json:"force,omitempty"`
}

type SaveAnchorParams struct {
	Summary string `json:"summary"`
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

func (s *Server) handleHybridSearch(ctx context.Context, req *mcp.CallToolRequest, args HybridSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing query"}},
			IsError: true,
		},
		nil, nil
	}

	limit := args.Limit
	if limit == 0 {
		limit = 20
	}

	res, err := s.engine.HybridSearch(ctx, args.Query, limit, args.Offset)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Hybrid search failed: %v", err)}},
			IsError: true,
		},
		nil, nil
	}
	out, err := json.Marshal(res)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal hybrid search results: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	tthought := fmt.Sprintf("<thought>\nExecuted hybrid search for '%s'. Found %d AST symbols and %d Engram insights (limit: %d, offset: %d).\n</thought>\n", args.Query, len(res.Symbols), len(res.Insights), limit, args.Offset)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: tthought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleCompactContext(ctx context.Context, req *mcp.CallToolRequest, args CompactContextParams) (*mcp.CallToolResult, any, error) {
	// [Strike 5] Predictive Context: Identify critical hotspots for high-fidelity summary
	systemPrompt := CompactContextSystemPrompt
	cmd, err := utils.SafeCommand(ctx, "git", "diff", "HEAD", "--unified=0")
	if err == nil {
		diffOut, err := cmd.Output()
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
	}

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

	// Invoke Engram CLI autonomously
	cmd, err := utils.SafeCommand(ctx, "engram", "save", "--title", title, "--type", "session_summary", "--project", project, "--", engramContent)
	if err != nil {
		s.logger.Warn("Failed to create safe Engram command", "error", err)
	} else if err := cmd.Run(); err != nil {
		s.logger.Warn("Failed to persist anchor to Engram, using local fallback", "error", err)

		scouterDir := ".scouter"
		os.MkdirAll(scouterDir, 0755)
		anchorPath := filepath.Join(scouterDir, "anchor.md")
		header := fmt.Sprintf("# 🏛️ SCOUTER ANCHOR (Local Fallback)\n*Compacted on: %s*\n\n", now)
		os.WriteFile(anchorPath, []byte(header+args.Summary), 0644)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "⚠️ Engram save failed. Anchor saved to local fallback: " + anchorPath},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Latent memory anchored in Engram for project: " + project},
		},
	}, nil, nil
}
