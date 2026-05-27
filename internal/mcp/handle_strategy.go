package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Strategy Param structs

type JudgeParams struct {
	Diff     string `json:"diff,omitempty" jsonschema:"Optional: Git diff of the changes to judge"`
	Proposal string `json:"proposal,omitempty" jsonschema:"REQUIRED (if diff is empty). The architectural proposal or intent to audit"`
}

type ExploreSDDParams struct {
	Query  string `json:"query,omitempty" jsonschema:"Optional: Search query to filter SDD artifacts"`
	Type   string `json:"type,omitempty" jsonschema:"Optional: Filter by SDD type ('roadmap', 'tasks', 'specs')"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Optional: Max results to return (default: 10)"`
	Offset int    `json:"offset,omitempty" jsonschema:"Optional: Number of results to skip for pagination"`
}

// Handlers

func (s *Server) handleJudge(ctx context.Context, req *mcp.CallToolRequest, args JudgeParams) (*mcp.CallToolResult, any, error) {
	engramCtx := s.fetchEngramContext(ctx, "architecture decisions ADR "+args.Proposal)

	prompt := fmt.Sprintf("Architectural Proposal: %s\n\nGit Diff:\n%s\n\nHistorical Context:\n%s", args.Proposal, args.Diff, engramCtx)

	type judgeRes struct {
		text   string
		rating float64
		err    error
	}

	results := make(chan judgeRes, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	judgeFunc := func() {
		defer wg.Done()
		samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
			SystemPrompt: JudgeSystemPrompt,
			Messages: []*mcp.SamplingMessage{
				{Role: "user", Content: &mcp.TextContent{Text: prompt}},
			},
			MaxTokens: 2048,
		})
		if err != nil {
			results <- judgeRes{err: err}
			return
		}
		txt, ok := samplingRes.Content.(*mcp.TextContent)
		if !ok {
			results <- judgeRes{err: fmt.Errorf("unexpected sampling response type")}
			return
		}
		rating, _ := utils.ParseRating(txt.Text)
		results <- judgeRes{text: txt.Text, rating: rating}
	}

	go judgeFunc()
	go judgeFunc()

	wg.Wait()
	close(results)

	var texts []string
	var ratings []float64
	var allFindings []string

	for r := range results {
		if r.err != nil {
			return nil, nil, fmt.Errorf("judge sampling failed: %w", r.err)
		}
		texts = append(texts, r.text)
		ratings = append(ratings, r.rating)
		allFindings = append(allFindings, utils.ExtractList(r.text, "Findings")...)
	}

	// Synthesis
	avgRating := (ratings[0] + ratings[1]) / 2.0
	divergence := math.Abs(ratings[0] - ratings[1])
	convergence := divergence <= 2.0

	verdict := "HAKAI"
	if avgRating >= 9.0 {
		verdict = "SOVEREIGN"
	} else if avgRating >= 8.0 {
		verdict = "REDEMPTION"
	}

	convergenceStatus := "CONVERGED"
	if !convergence {
		convergenceStatus = "CONTESTED"
	}

	report := fmt.Sprintf("# ⚖️ DIVINE VERDICT: %s\n\n", verdict)
	report += fmt.Sprintf("**Average Rating**: %.1f / 10.0\n", avgRating)
	report += fmt.Sprintf("**Convergence**: %s (Divergence: %.1f)\n\n", convergenceStatus, divergence)
	report += "## Consolidated Findings\n"
	for _, f := range allFindings {
		report += fmt.Sprintf("- %s\n", f)
	}

	report += "\n---\n"
	report += "### Judge A Raw\n" + texts[0] + "\n"
	report += "\n---\n"
	report += "### Judge B Raw\n" + texts[1] + "\n"

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: report},
		},
	}, nil, nil
}

func (s *Server) handleExploreSDD(ctx context.Context, req *mcp.CallToolRequest, args ExploreSDDParams) (*mcp.CallToolResult, any, error) {
	var result any
	var err error

	limit := args.Limit
	if limit == 0 {
		limit = 10
	}

	switch args.Type {
	case "roadmap":
		result, err = s.engine.GetSDDRoadmap(ctx)
	case "tasks":
		result, err = s.engine.GetSDDTasks(ctx)
	case "specs", "":
		result, err = s.engine.SearchSDDSpecs(ctx, args.Query, limit, args.Offset)
	default:
		return nil, nil, fmt.Errorf("unknown SDD type: %s", args.Type)
	}

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("SDD exploration failed: %v", err)}}, 
			IsError: true,
		}, nil, nil
	}

	out, _ := json.Marshal(result)
	if string(out) == "null" {
		out = []byte("[]")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}
