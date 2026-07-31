package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"encoding/json"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ImpactParams struct {
	SymbolName string `json:"symbolName" jsonschema:"REQUIRED. The name of the symbol to analyze"`
	FilePath   string `json:"filePath" jsonschema:"REQUIRED. Path to the file containing the symbol"`
	MaxDepth   int    `json:"maxDepth,omitempty" jsonschema:"Optional: Maximum recursion depth for impact analysis"`
	Verbose    bool   `json:"verbose,omitempty" jsonschema:"Optional: Include detailed metrics and Mermaid graph"`
}

type CriticalParams struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"Optional: Max critical symbols to return (default: 10, max: 50)"`
	Format string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
}

type ObsidianExportParams struct {
	SymbolName string `json:"symbolName" jsonschema:"REQUIRED. The name of the symbol to export"`
	FilePath   string `json:"filePath" jsonschema:"REQUIRED. Path to the file containing the symbol"`
	VaultPath  string `json:"vaultPath,omitempty" jsonschema:"Optional: Custom path for the Obsidian vault export"`
}

type PredictParams struct {
	Diff string `json:"diff,omitempty" jsonschema:"Optional: Git diff to analyze (defaults to uncommitted changes)"`
}

type LintArchitectureParams struct {
	TargetPath string `json:"target_path" jsonschema:"REQUIRED. The target path to lint"`
}

func (s *Server) handleImpact(ctx context.Context, req *mcp.CallToolRequest, args ImpactParams) (*mcp.CallToolResult, any, error) {
	risk, err := s.diagnostic.AssessRisk(ctx, args.SymbolName, args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to assess risk: %v", err)}},
				IsError: true,
			},
			nil, nil
	}
	messenger := &mcpMessenger{server: s, req: req}
	if risk.RiskScore >= 0.8 && messenger != nil {
		prompt := fmt.Sprintf("The function '%s' in '%s' has a CRITICAL Risk Score of %.4f. Based on its centrality and blast radius, please provide a brief architectural refactoring proposal to reduce its impact.", args.SymbolName, args.FilePath, risk.RiskScore)
		_, err := messenger.Ask(ctx, "You are an expert software architect.", prompt)
		if err != nil && s.logger != nil {
			s.logger.Error("oracle ask failed", "error", err)
		}
	}

	res, err := s.impact.Analyze(ctx, args.SymbolName, args.FilePath, 5)
	if err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to analyze impact: %v", err)}},
				IsError: true,
			},
			nil, nil
	}
	outStr := engine.EncodeZONImpact(res)

	thought := fmt.Sprintf("Calculated blast radius for '%s'. Target risk score: %.4f (Level: %s). Found %d affected callers.", args.SymbolName, res.Target.RiskScore, res.RiskLevel, len(res.Callers))

	return formatTextResult(thought, outStr), nil, nil
}

func (s *Server) handleCritical(ctx context.Context, req *mcp.CallToolRequest, args CriticalParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	results, err := s.analyzer.GetCriticalSymbols(ctx, limit)
	if err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to get critical symbols: %v", err)}},
				IsError: true,
			},
			nil, nil
	}

	thought := fmt.Sprintf("Risk Analysis: Identifying high-risk symbols (high centrality and fragility). Found %d targets (limit: %d).",
		len(results), limit)

	res, err := formatResult(thought, results)
	return res, nil, err
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

	res, _ := s.impact.Analyze(ctx, args.SymbolName, args.FilePath, 5)

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
		cmd, err := utils.SafeCommand(ctx, "git", "diff", "HEAD", "--unified=0")
		if err == nil {
			if out, cmdErr := cmd.Output(); cmdErr == nil {
				diff = string(out)
			}
		}
	}

	results, err := s.impact.PredictTests(ctx, diff)
	if err != nil {
		return nil, nil, err
	}

	outStr := engine.EncodeZONPredict(results)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: outStr},
		},
	}, nil, nil
}

func (s *Server) handleLintArchitecture(ctx context.Context, req *mcp.CallToolRequest, args LintArchitectureParams) (*mcp.CallToolResult, any, error) {
	results, err := s.astRules.Audit(ctx, args.TargetPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to audit architecture: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	outBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(outBytes)},
		},
	}, nil, nil
}
