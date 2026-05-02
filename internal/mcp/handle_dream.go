package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Rogercode97/scouter/internal/adapters/llm"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DreamParams struct {
	Project string `json:"project,omitempty"`
	Hours   int    `json:"hours,omitempty"`
}

type KnowledgeGraphParams struct {
	SymbolName string `json:"symbolName"`
}

func (s *Server) handleDream(ctx context.Context, req *mcp.CallToolRequest, args DreamParams) (*mcp.CallToolResult, any, error) {
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

	// Create a new Distiller for this specific request to use the current session
	distiller := llm.NewMCPDistiller(req.Session)
	
	// We use the pre-initialized appService but with a fresh distiller for sampling
	// In a real production scenario, we might want to inject the distiller into a 
	// request-scoped service, but for now we'll just use the session-bound distiller.
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

// postToolHook executes passive memory extraction after high-impact tool calls.
// REQ-4: Heuristic and background execution.
func (s *Server) postToolHook(req *mcp.CallToolRequest, impact bool) {
	if !impact {
		return
	}

	// Run in background to not block the tool response
	go func() {
		// Use a background context with timeout for distillation
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		project := "scouter" // Default project name for now

		s.logger.Debug("triggering passive distillation hook", "project", project)

		// Ensure we have a distiller configured for sampling
		distiller := llm.NewMCPDistiller(req.Session)
		s.appService.SetDistiller(distiller)

		// REQ-5: Extract transcript from session (if supported by implementation)
		transcript := s.GetTranscript(req)

		err := s.appService.PassiveDistill(ctx, project, transcript)
		if err != nil {
			s.logger.Warn("Passive distillation failed", "error", err)
		} else {
			s.logger.Debug("Passive distillation successful")
		}
	}()
}

// GetTranscript attempts to retrieve the current session transcript.
// In a real MCP implementation, this would query the session history.
// For now, we return a placeholder or empty slice as the SDK doesn't expose this directly yet.
func (s *Server) GetTranscript(req *mcp.CallToolRequest) []memory.Message {
	// TODO: Integrate with a real history provider if available
	return []memory.Message{}
}

func (s *Server) handleKnowledgeGraph(ctx context.Context, req *mcp.CallToolRequest, args KnowledgeGraphParams) (*mcp.CallToolResult, any, error) {
	if args.SymbolName == "" {
		return nil, nil, fmt.Errorf("missing symbolName")
	}

	// Invoke Engram CLI search
	cmd := exec.CommandContext(ctx, "engram", "search", "--query", args.SymbolName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("engram search failed: %w\n%s", err, string(out))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}
