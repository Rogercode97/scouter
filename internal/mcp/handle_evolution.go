package mcp

import (
	"context"
	"fmt"

	"strings"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Evolution Param structs

type SelfHealParams struct {
	ErrorLog string `json:"errorLog" jsonschema:"REQUIRED. The raw error log or test failure output containing the failure context"`
}

type RippleRefactorParams struct {
	SymbolName     string `json:"symbolName" jsonschema:"REQUIRED. The name of the symbol to refactor"`
	Transformation string `json:"transformation" jsonschema:"REQUIRED. The structural transformation to apply (e.g., 'rename:NewName')"`
}

type EvolveParams struct {
	Proposal string `json:"proposal" jsonschema:"REQUIRED. The multi-file evolution proposal in natural language detailing the desired architecture change"`
	Force    bool   `json:"force,omitempty" jsonschema:"Optional: Bypass safety guardrails for core file modifications"`
}

type CommitParams struct{}

type RollbackParams struct{}

type DiffParams struct{}

// Handlers

func (s *Server) handleSelfHeal(ctx context.Context, req *mcp.CallToolRequest, args SelfHealParams) (*mcp.CallToolResult, any, error) {
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

	// [Sovereignty Upgrade] Inline TruthEngine logic
	report, err := s.diagnostic.Diagnose(ctx, args.ErrorLog)
	if err != nil {
		return nil, nil, fmt.Errorf("diagnose failed: %w", err)
	}

	messenger := &healerMessenger{server: s, req: req, engramCtx: engramCtx}
	s.healer.DoFixRequest = func(fCtx context.Context, prompt string) (string, error) {
		enrichedPrompt := "Historical Insights:\n" + strings.Join(report.Insights, "\n") + "\n\n" + prompt
		return messenger.Ask(fCtx, "You are an autonomous Go fixing agent.", enrichedPrompt)
	}

	res, err := s.healer.Fix(ctx, report.ErrorLog)
	if err != nil {
		return nil, nil, fmt.Errorf("self-heal failed: %w", err)
	}

	outStr := fmt.Sprintf("Status: %s\nFile: %s\nFixed Code:\n%s\nTest Output:\n%s", res.Status, res.Metadata["failingFile"], res.FixedCode, res.TestOutput)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: outStr},
		},
	}, nil, nil
}

func (s *Server) handleRippleRefactor(ctx context.Context, req *mcp.CallToolRequest, args RippleRefactorParams) (*mcp.CallToolResult, any, error) {
	if args.SymbolName == "" || args.Transformation == "" {
		return nil, nil, fmt.Errorf("missing symbolName or transformation")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Delegate to TruthEngine
	res, err := s.evolution.Propagate(ctx, args.SymbolName, args.Transformation, &mcpMessenger{server: s, req: req})
	if err != nil {
		return nil, nil, fmt.Errorf("propagation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
		},
	}, nil, nil
}

func (s *Server) handleCommit(ctx context.Context, req *mcp.CallToolRequest, args CommitParams) (*mcp.CallToolResult, any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.evolution.CommitLedger(ctx)
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

func (s *Server) handleRollback(ctx context.Context, req *mcp.CallToolRequest, args RollbackParams) (*mcp.CallToolResult, any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.evolution.RollbackLedger(ctx)
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

func (s *Server) handleDiff(ctx context.Context, req *mcp.CallToolRequest, args DiffParams) (*mcp.CallToolResult, any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, err := s.evolution.GetLedgerDiff(ctx)
	if err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Diff failed: %v", err)}},
				IsError: true,
			},
			nil, nil
	}

	summary := s.evolution.GetLedgerSummary(ctx)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: summary + "\n\n" + res},
		},
	}, nil, nil
}

func (s *Server) handleEvolve(ctx context.Context, req *mcp.CallToolRequest, args EvolveParams) (*mcp.CallToolResult, any, error) {
	if args.Proposal == "" {
		return nil, nil, fmt.Errorf("missing proposal")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Log user prompt
	s.AppendSessionMessage(memory.Message{Role: "user", Content: args.Proposal})

	res, err := s.evolution.ProposeEvolution(ctx, args.Proposal, args.Force, &mcpMessenger{server: s, req: req})
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
		},
	}, nil, nil
}
