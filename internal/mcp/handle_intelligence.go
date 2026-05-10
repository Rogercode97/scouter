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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ImpactParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	MaxDepth   int    `json:"maxDepth,omitempty"`
	Verbose    bool   `json:"verbose,omitempty"`
}

type CriticalParams struct {
	Limit int `json:"limit,omitempty"`
}

type ObsidianExportParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	VaultPath  string `json:"vaultPath,omitempty"`
}

type PredictParams struct {
        Diff string `json:"diff,omitempty"`
}

type FindLogicalTwinParams struct {
        SymbolName string `json:"symbolName"`
        FilePath   string `json:"filePath"`
}
func (s *Server) handleImpact(ctx context.Context, req *mcp.CallToolRequest, args ImpactParams) (*mcp.CallToolResult, any, error) {
	// [Sovereignty Upgrade] Route through TruthEngine
	res, err := s.engine.AnalyzeImpact(ctx, args.SymbolName, args.FilePath, args.Verbose, &mcpMessenger{req: req})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to analyze impact: %v", err)}},
			IsError: true,
		},
		nil, nil
	}
	out, err := json.Marshal(res)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal impact result: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	thought := fmt.Sprintf("<thought>\nCalculated blast radius for '%s'. Target risk score: %.4f (Level: %s). Found %d affected callers.\n</thought>\n", args.SymbolName, res.Target.RiskScore, res.RiskLevel, len(res.Callers))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleCritical(ctx context.Context, req *mcp.CallToolRequest, args CriticalParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	results, err := s.engine.GetCriticalSymbols(ctx, limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to get critical symbols: %v", err)}},
			IsError: true,
		},
		nil, nil
	}
	out, _ := json.Marshal(results)

	thought := fmt.Sprintf("<thought>\nRisk Analysis: Identifying high-risk symbols (high centrality and fragility). Found %d targets (limit: %d).\n</thought>\n", len(results), limit)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleObsidianExport(ctx context.Context, req *mcp.CallToolRequest, args ObsidianExportParams) (*mcp.CallToolResult, any, error) {
	if args.SymbolName == "" || args.FilePath == "" {
		return nil, nil, fmt.Errorf("missing symbolName or filePath")
	}

	// [Sovereignty Fix] Path Traversal Armor (Moved to Top)
	exportPath := args.VaultPath
	if exportPath == "" {
		exportPath = "scouter_exports"
	}

cwd, _ := os.Getwd()
cleanPath := filepath.Clean(exportPath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(cwd, cleanPath)
	}
	if !strings.HasPrefix(cleanPath, cwd) {
		return nil, nil, fmt.Errorf("security violation: export path '%s' is outside the workspace", exportPath)
	}

	res, err := s.engine.AnalyzeImpact(ctx, args.SymbolName, args.FilePath, true, nil)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Format("2006-01-02")
	content := fmt.Sprintf(`---
symbol: %s
file: %s
risk_score: %.2f
risk_level: %s
historical_bugfixes: %d
date: %s
---
# Impact Analysis: [[%s]]

## Metadata
- **File**: %s
- **Risk Score**: %.4f
- **Risk Level**: %s
- **Historical Bugfixes**: %d (from Engram)

## Blast Radius (Mermaid)
%s%s%s

## Affected Callers
`,
		args.SymbolName, args.FilePath, res.Target.RiskScore, res.RiskLevel, res.Target.Metrics.HistoricalBugfixes, now,
		args.SymbolName, args.FilePath, res.Target.RiskScore, res.RiskLevel, res.Target.Metrics.HistoricalBugfixes,
		"```mermaid\n", res.Mermaid, "\n```")

	for _, caller := range res.Callers {
		content += fmt.Sprintf("- [[%s]] (%s, distance: %d)\n", caller.Symbol, caller.File, caller.Distance)
	}

	if err := os.MkdirAll(cleanPath, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	fileName := fmt.Sprintf("Impact-%s.md", strings.ReplaceAll(args.SymbolName, ":", "_"))
	fullPath := filepath.Join(cleanPath, fileName)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, nil, fmt.Errorf("failed to write obsidian note: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Obsidian note exported to: " + fullPath},
		},
	}, nil, nil
}

func (s *Server) handlePredict(ctx context.Context, req *mcp.CallToolRequest, args PredictParams) (*mcp.CallToolResult, any, error) {
	diff := args.Diff
	if diff == "" {
		out, err := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0").Output()
		if err == nil {
			diff = string(out)
		}
	}

	results, err := s.engine.PredictTests(ctx, diff)
	if err != nil {
		return nil, nil, err
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}


func (s *Server) handleFindLogicalTwin(ctx context.Context, req *mcp.CallToolRequest, args FindLogicalTwinParams) (*mcp.CallToolResult, any, error) {
        if args.SymbolName == "" || args.FilePath == "" {
                return nil, nil, fmt.Errorf("missing symbolName or filePath")
        }

        results, err := s.engine.FindLogicalTwins(ctx, args.SymbolName, args.FilePath)
        if err != nil {
                return &mcp.CallToolResult{
                        Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to find logical twins: %v", err)}},
                        IsError: true,
                }, nil, nil
        }

        out, _ := json.Marshal(results)
        thought := fmt.Sprintf("<thought>\nStructural Analysis: Identifying symbols with identical logical signatures to '%s'. Found %d twins.\n</thought>\n", args.SymbolName, len(results))

        return &mcp.CallToolResult{
                Content: []mcp.Content{
                        &mcp.TextContent{Text: thought + string(out)},
                },
        }, nil, nil
}
