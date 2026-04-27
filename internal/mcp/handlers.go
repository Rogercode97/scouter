package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool param structs for type safety

type IndexParams struct {
	FilePath string `json:"filePath"`
}

type SearchParams struct {
	Query string `json:"query"`
	Type  string `json:"type,omitempty"`
}

type HybridSearchParams struct {
	Query string `json:"query"`
}

type ReadParams struct {
	FilePath string `json:"filePath"`
	Pointer  string `json:"pointer"`
}

type CallersParams struct {
	CalleeName string `json:"calleeName"`
}

type ImpactParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	MaxDepth   int    `json:"maxDepth,omitempty"`
}

type CriticalParams struct {
	Limit int `json:"limit,omitempty"`
}

type DependenciesParams struct{}

type StructuralSearchParams struct {
	Pattern string `json:"pattern"`
	Ext     string `json:"ext"`
	Path    string `json:"path,omitempty"`
}

type PureSignalParams struct {
	Text  string `json:"text"`
	Mode  string `json:"mode,omitempty"`
	Level string `json:"level,omitempty"`
}

type ObsidianExportParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	VaultPath  string `json:"vaultPath,omitempty"`
}

// Handlers adapted to mcp.AddTool signature

func (s *Server) handleIndex(ctx context.Context, req *mcp.CallToolRequest, args IndexParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return nil, nil, fmt.Errorf("missing filePath")
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return nil, nil, err
	}

	_, _, err = engine.ParseFile(ctx, path, nil)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Indexed " + args.FilePath},
		},
	}, nil, nil
}

func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	results, err := s.store.SearchSymbols(ctx, args.Query, args.Type)
	if err != nil {
		return nil, nil, err
	}

	if len(results) > 500 {
		results = results[:500]
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal search results: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}

func (s *Server) handleRead(ctx context.Context, req *mcp.CallToolRequest, args ReadParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" || args.Pointer == "" {
		return nil, nil, fmt.Errorf("missing filePath or pointer")
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return nil, nil, err
	}

	rng, err := s.resolver.Resolve(ctx, path, args.Pointer)
	if err != nil {
		return nil, nil, err
	}

	content, err := engine.ReadFragment(ctx, path, rng)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

func (s *Server) handleCallers(ctx context.Context, req *mcp.CallToolRequest, args CallersParams) (*mcp.CallToolResult, any, error) {
	if args.CalleeName == "" {
		return nil, nil, fmt.Errorf("missing calleeName")
	}
	results, err := s.store.GetCallers(ctx, args.CalleeName)
	if err != nil {
		return nil, nil, err
	}
	if len(results) > 500 {
		results = results[:500]
	}
	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleImpact(ctx context.Context, req *mcp.CallToolRequest, args ImpactParams) (*mcp.CallToolResult, any, error) {
	maxDepth := args.MaxDepth
	if maxDepth == 0 {
		maxDepth = 5
	}
	res, err := s.store.GetImpact(ctx, args.SymbolName, args.FilePath, maxDepth)
	if err != nil {
		return nil, nil, err
	}
	
	// Limit callers if they exceed 500
	if len(res.Callers) > 500 {
		res.Callers = res.Callers[:500]
	}

	out, err := json.Marshal(res)
	if err != nil {
		return nil, nil, err
	}

	// 4. [Divine Synergy] Sampling Oracle
	// If Risk is Critical (>0.8), request a refactoring proposal from the Model via MCP Sampling.
	if res.Target.RiskScore >= 0.8 {
		samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
			Messages: []*mcp.SamplingMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("The function '%s' in '%s' has a CRITICAL Risk Score of %.4f. Based on its centrality and blast radius, please provide a brief architectural refactoring proposal to reduce its impact.", args.SymbolName, args.FilePath, res.Target.RiskScore),
					},
				},
			},
			MaxTokens: 1024,
		})
		if err == nil {
			if txt, ok := samplingRes.Content.(*mcp.TextContent); ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: string(out)},
						&mcp.TextContent{Text: "\n\n--- 🔮 ORACLE REFACTORING PROPOSAL ---\n" + txt.Text},
					},
				}, nil, nil
			}
		} else {
			s.logger.Warn("Sampling Oracle failed", "error", err)
		}
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleCritical(ctx context.Context, req *mcp.CallToolRequest, args CriticalParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 500 {
		limit = 500
	}
	results, err := s.store.GetCriticalSymbols(ctx, limit)
	if err != nil {
		return nil, nil, err
	}
	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleDependencies(ctx context.Context, req *mcp.CallToolRequest, args DependenciesParams) (*mcp.CallToolResult, any, error) {
	res, err := s.store.GetDependencies(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(res) > 500 {
		res = res[:500]
	}
	out, err := json.Marshal(res)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleStructuralSearch(ctx context.Context, req *mcp.CallToolRequest, args StructuralSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" || args.Ext == "" {
		return nil, nil, fmt.Errorf("missing pattern or ext")
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = "."
	}

	path, err := utils.ValidatePath(searchPath)
	if err != nil {
		return nil, nil, err
	}

	results, err := engine.StructuralSearch(ctx, path, args.Pattern, args.Ext)
	if err != nil {
		return nil, nil, err
	}

	if len(results) > 500 {
		results = results[:500]
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
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

	res, err := s.store.GetImpact(ctx, args.SymbolName, args.FilePath, 5)
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
`, args.SymbolName, args.FilePath, res.Target.RiskScore, res.RiskLevel, res.Target.Metrics.HistoricalBugfixes, now,
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

func (s *Server) handleHybridSearch(ctx context.Context, req *mcp.CallToolRequest, args HybridSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return nil, nil, fmt.Errorf("missing query")
	}

	// Execute AST and Engram searches in parallel
	type symRes struct {
		symbols []store.Symbol
		err     error
	}
	type insRes struct {
		insights []types.MemoryInsight
		err      error
	}

	symChan := make(chan symRes, 1)
	insChan := make(chan insRes, 1)

	go func() {
		res, err := s.store.SearchSymbols(ctx, args.Query, "")
		symChan <- symRes{res, err}
	}()

	go func() {
		res, err := s.store.GetMemoryInsights(ctx, args.Query)
		insChan <- insRes{res, err}
	}()

	sRes := <-symChan
	if sRes.err != nil {
		return nil, nil, fmt.Errorf("AST search failed: %w", sRes.err)
	}

	iRes := <-insChan
	if iRes.err != nil {
		return nil, nil, fmt.Errorf("Engram search failed: %w", iRes.err)
	}

	// Map store.Symbol to types.Symbol
	var symbols []types.Symbol
	for _, s := range sRes.symbols {
		symbols = append(symbols, types.Symbol{
			Name:      s.Name,
			Type:      s.Type,
			Signature: s.Signature,
			Doc:       s.Doc,
			Path:      s.Path,
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
		})
	}

	result := types.HybridSearchResult{
		Symbols:  symbols,
		Insights: iRes.insights,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal hybrid results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}
