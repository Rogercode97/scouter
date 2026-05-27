package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Evolution Param structs

type ApplyArgs struct {
	Action       string `json:"action" jsonschema:"The action to perform (ripple|evolve)"`
	FilePath     string `json:"filePath,omitempty" jsonschema:"Optional: Path to the file containing the target symbol"`
	TargetSymbol string `json:"targetSymbol,omitempty" jsonschema:"Optional: The target symbol to refactor (for ripple)"`
	Instructions string `json:"instructions,omitempty" jsonschema:"The multi-file evolution proposal or transformation instructions"`
	Force        bool   `json:"force,omitempty" jsonschema:"Optional: Bypass safety guardrails for core file modifications"`
}

type SelfHealParams struct {
	ErrorLog string `json:"errorLog" jsonschema:"The raw error log or test failure output"`
}

type RippleRefactorParams struct {
	SymbolName     string `json:"symbolName" jsonschema:"The name of the symbol to refactor"`
	Transformation string `json:"transformation" jsonschema:"The structural transformation to apply (e.g., 'rename:NewName')"`
}

type EvolveParams struct {
	Proposal string `json:"proposal" jsonschema:"The multi-file evolution proposal in natural language"`
	Force    bool   `json:"force,omitempty" jsonschema:"Optional: Bypass safety guardrails for core file modifications"`
}

type CommitParams struct{}

type RollbackParams struct{}

type DiffParams struct{}

// Handlers

func (s *Server) HandleApply(ctx context.Context, req *mcp.CallToolRequest, args ApplyArgs) (*mcp.CallToolResult, any, error) {
	switch args.Action {
	case "ripple":
		if args.TargetSymbol == "" || args.Instructions == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "targetSymbol and instructions (transformation) required for ripple"}}, IsError: true}, nil, nil
		}
		return s.executeRippleRefactor(ctx, req, RippleRefactorParams{SymbolName: args.TargetSymbol, Transformation: args.Instructions})
	case "evolve":
		if args.Instructions == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "instructions (proposal) required for evolve"}}, IsError: true}, nil, nil
		}
		return s.executeEvolve(ctx, req, EvolveParams{Proposal: args.Instructions, Force: args.Force})
	default:
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invalid action for apply"}}, IsError: true}, nil, nil
	}
}

func (s *Server) executeSelfHeal(ctx context.Context, req *mcp.CallToolRequest, args SelfHealParams) (*mcp.CallToolResult, any, error) {
	if args.ErrorLog == "" {
		return nil, nil, fmt.Errorf("missing errorLog")
	}

	// [Sovereignty Mandate] Serial execution via Mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	searchQuery := args.ErrorLog
	if len(searchQuery) > 100 {
		searchQuery = searchQuery[:100]
	}
	engramCtx := s.fetchEngramContext(ctx, "bugfix "+searchQuery)

	// Delegate to TruthEngine
	res, err := s.engine.Fix(ctx, args.ErrorLog, &healerMessenger{server: s, req: req, engramCtx: engramCtx})
	if err != nil {
		return nil, nil, fmt.Errorf("self-heal failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
		},
	}, nil, nil
}

func (s *Server) executeRippleRefactor(ctx context.Context, req *mcp.CallToolRequest, args RippleRefactorParams) (*mcp.CallToolResult, any, error) {
	if args.SymbolName == "" || args.Transformation == "" {
		return nil, nil, fmt.Errorf("missing symbolName or transformation")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Delegate to TruthEngine
	res, err := s.engine.Propagate(ctx, args.SymbolName, args.Transformation, &mcpMessenger{server: s, req: req})
	if err != nil {
		return nil, nil, fmt.Errorf("propagation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
		},
	}, nil, nil
}

func (s *Server) executeCommit(ctx context.Context, req *mcp.CallToolRequest, args CommitParams) (*mcp.CallToolResult, any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.engine.CommitLedger(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Commit failed: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: res}},
	}, nil, nil
}

func (s *Server) executeRollback(ctx context.Context, req *mcp.CallToolRequest, args RollbackParams) (*mcp.CallToolResult, any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.engine.RollbackLedger(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Rollback failed: %v", err)}},
			IsError: true,
		},
		nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: res}},
	}, nil, nil
}

func (s *Server) executeDiff(ctx context.Context, req *mcp.CallToolRequest, args DiffParams) (*mcp.CallToolResult, any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, err := s.engine.GetLedgerDiff(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Diff failed: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	summary := s.engine.GetLedgerSummary(ctx)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: summary + "\n\n" + res},
		},
	}, nil, nil
}

func (s *Server) executeEvolve(ctx context.Context, req *mcp.CallToolRequest, args EvolveParams) (*mcp.CallToolResult, any, error) {
	if args.Proposal == "" {
		return nil, nil, fmt.Errorf("missing proposal")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Log user prompt
	s.AppendSessionMessage(memory.Message{Role: "user", Content: args.Proposal})

	// 1. Sampling: Request Genome Mutation
	samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: GEPSystemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: args.Proposal}},
		},
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sampling evolution failed: %w", err)
	}

	txt, ok := samplingRes.Content.(*mcp.TextContent)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected sampling response type")
	}

	// Log assistant response
	s.AppendSessionMessage(memory.Message{Role: "assistant", Content: txt.Text})

	// 2. Parse JSON Mutations
	var mutations []struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	rawJSON := utils.ExtractJSON(txt.Text)
	if err := json.Unmarshal([]byte(rawJSON), &mutations); err != nil {
		return nil, nil, fmt.Errorf("failed to parse mutation JSON: %w\nRaw: %s", err, txt.Text)
	}

	// 3. Stage in Ledger
	stagedCount := 0
	for _, m := range mutations {
		if !args.Force && strings.Contains(m.File, "internal/mcp/handlers.go") {
			return nil, nil, fmt.Errorf("SOVEREIGNTY VIOLATION: Mutation attempts to modify GEP core logic in '%s'. Use 'force:true' if this is an intended self-lobotomy.", m.File)
		}

		if err := s.engine.StageMutation(ctx, m.File, m.Content); err != nil {
			return nil, nil, fmt.Errorf("failed to stage mutation for %s: %w", m.File, err)
		}
		stagedCount++
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("✅ Evolution staged in Ledger for %d files. Use 'scouter_diff' to review and 'scouter_commit' to apply changes.", stagedCount)},
		},
	}, nil, nil
}
