package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/adapters/llm"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ContextArgs struct {
	Mode   string `json:"mode" jsonschema:"The mode to run (compact|pure_signal)"`
	Budget int    `json:"budget,omitempty" jsonschema:"Optional: The token budget for compaction"`
	Filter string `json:"filter,omitempty" jsonschema:"Optional: Filter for pure signal"`
	Text   string `json:"text,omitempty" jsonschema:"Optional: Text to filter for pure signal"`
}

type MemoryArgs struct {
	Action     string `json:"action" jsonschema:"The memory action (dream|save_anchor)"`
	Context    string `json:"context" jsonschema:"The context or summary to save/process"`
	AnchorName string `json:"anchorName,omitempty" jsonschema:"Optional: The name of the anchor for save_anchor"`
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

func (s *Server) HandleContext(ctx context.Context, req *mcp.CallToolRequest, args ContextArgs) (*mcp.CallToolResult, any, error) {
	switch args.Mode {
	case "compact":
		force := false // Assuming Budget or something could represent force in new consolidation. For now just false.
		return s.executeCompactContext(ctx, req, CompactContextParams{Force: force})
	case "pure_signal":
		if args.Text == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "text required for pure_signal mode"}}, IsError: true}, nil, nil
		}
		return s.executePureSignal(ctx, req, PureSignalParams{Text: args.Text, Level: args.Filter})
	default:
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invalid mode for context"}}, IsError: true}, nil, nil
	}
}

func (s *Server) HandleMemory(ctx context.Context, req *mcp.CallToolRequest, args MemoryArgs) (*mcp.CallToolResult, any, error) {
	switch args.Action {
	case "dream":
		project := args.Context
		if project == "" {
			project = "scouter"
		}
		// Hours could be parsed if we extended MemoryArgs, but for now 24
		hours := 24
		return s.executeDream(ctx, req, DreamParams{Project: project, Hours: hours})
	case "save_anchor":
		if args.Context == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "context (summary) required for save_anchor action"}}, IsError: true}, nil, nil
		}
		// args.AnchorName could be used if executeSaveAnchor took it. SaveAnchorParams only takes Summary.
		return s.executeSaveAnchor(ctx, req, SaveAnchorParams{Summary: args.Context})
	default:
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invalid action for memory"}}, IsError: true}, nil, nil
	}
}

func (s *Server) executeDream(ctx context.Context, req *mcp.CallToolRequest, args DreamParams) (*mcp.CallToolResult, any, error) {
	project := args.Project
	if project == "" {
		project = utils.GetRepoName(ctx)
	}
	if project == "" {
		project = "scouter"
	}
	hours := args.Hours
	if hours == 0 {
		hours = 24
	}

	distiller := llm.NewMCPDistiller(req.Session)
	s.appService.SetDistiller(distiller)

	err := s.appService.DistillAndSave(ctx, project, hours)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("✅ Distilled and saved memory for project %s (last %d hours)", project, hours)},
		},
	}, nil, nil
}

func (s *Server) executePureSignal(ctx context.Context, req *mcp.CallToolRequest, args PureSignalParams) (*mcp.CallToolResult, any, error) {
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



func (s *Server) executeCompactContext(ctx context.Context, req *mcp.CallToolRequest, args CompactContextParams) (*mcp.CallToolResult, any, error) {
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

func (s *Server) executeSaveAnchor(ctx context.Context, req *mcp.CallToolRequest, args SaveAnchorParams) (*mcp.CallToolResult, any, error) {
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
