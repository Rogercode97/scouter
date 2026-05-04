package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool param structs for type safety

type IndexParams struct {
	FilePath string `json:"filePath"`
}

type SearchParams struct {
	Query  string `json:"query"`
	Type   string `json:"type,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type HybridSearchParams struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type ReadParams struct {
	FilePath string `json:"filePath"`
	Pointer  string `json:"pointer"`
}

type CallersParams struct {
	CalleeName string `json:"calleeName"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

type DefinitionParams struct {
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`      // 1-based (standard for humans/agents)
	Character int    `json:"character"` // 1-based
}

type TypeInfoParams struct {
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

type ImpactParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	MaxDepth   int    `json:"maxDepth,omitempty"`
	Verbose    bool   `json:"verbose,omitempty"`
}

type CriticalParams struct {
	Limit int `json:"limit,omitempty"`
}

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

type SelfHealParams struct {
	ErrorLog string `json:"errorLog"`
}

type RippleRefactorParams struct {
	SymbolName     string `json:"symbolName"`
	Transformation string `json:"transformation"`
}

type ObsidianExportParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	VaultPath  string `json:"vaultPath,omitempty"`
}

type CompactContextParams struct {
	Force bool `json:"force,omitempty"`
}

type EvolveParams struct {
	Proposal string `json:"proposal"`
	Force    bool   `json:"force,omitempty"`
}

type PredictParams struct {
	Diff string `json:"diff,omitempty"`
}

type JudgeParams struct {
	Diff     string `json:"diff,omitempty"`
	Proposal string `json:"proposal,omitempty"`
}

type JudgeResult struct {
	Rating      float64  `json:"rating"`
	Verdict     string   `json:"verdict"` // SOVEREIGN, REDEMPTION, HAKAI
	Findings    []string `json:"findings"`
	RiskVectors []string `json:"risk_vectors"`
	Convergence bool     `json:"convergence"`
}

// Handlers adapted to mcp.AddTool signature

func (s *Server) handleIndex(ctx context.Context, req *mcp.CallToolRequest, args IndexParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing filePath"}},
			IsError: true,
		}, nil, nil
	}

	thought := fmt.Sprintf("<thought>\nIndexing AST symbols for path: %s. This will update the global call graph and enable precise impact analysis.\n</thought>\n", args.FilePath)

	if err := s.engine.Index(ctx, args.FilePath); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Indexing failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + fmt.Sprintf("✅ Indexed %s and updated global call graph", args.FilePath)},
		},
	}, nil, nil
}

func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // Sovereign Limit
	}
	
	results, err := s.store.SearchSymbols(ctx, args.Query, args.Type, limit, args.Offset)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search execution failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	out, _ := json.Marshal(results)
	thought := fmt.Sprintf("<thought>\nSovereign Search: Querying AST for '%s' (%s). Pagination: [Limit:%d Offset:%d]. Found %d matches.\n</thought>\n", 
		args.Query, args.Type, limit, args.Offset, len(results))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleRead(ctx context.Context, req *mcp.CallToolRequest, args ReadParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" || args.Pointer == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing filePath or pointer"}},
			IsError: true,
		}, nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	// [RTK Muscle] Delegation check
	if _, err := exec.LookPath("rtk"); err == nil {
		cmd := exec.CommandContext(ctx, "rtk", "read", path, "--pointer", args.Pointer, "--ultra-compact")
		if out, err := cmd.CombinedOutput(); err == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("<thought>\nDelegated read to RTK for Pure Signal extraction (pointer: %s).\n</thought>\n%s", args.Pointer, string(out))},
				},
			}, nil, nil
		}
		// If RTK fails, fallback to manual read
		if s.logger != nil {
			s.logger.Warn("RTK delegation failed, falling back to manual read", "error", err)
		}
	}

	rng, err := s.resolver.Resolve(ctx, path, args.Pointer)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	content, err := engine.ReadFragment(ctx, path, rng)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("<thought>\nManual read for pointer '%s'.\n</thought>\n%s", args.Pointer, content)},
		},
	}, nil, nil
}

func (s *Server) handleCallers(ctx context.Context, req *mcp.CallToolRequest, args CallersParams) (*mcp.CallToolResult, any, error) {
	if args.CalleeName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing calleeName"}},
			IsError: true,
		}, nil, nil
	}
	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	results, err := s.store.GetCallers(ctx, args.CalleeName, limit, args.Offset)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to get callers: %v", err)}},
			IsError: true,
		}, nil, nil
	}
	out, _ := json.Marshal(results)

	thought := fmt.Sprintf("<thought>\nCall Hierarchy: Mapping callers for '%s'. Pagination: [Limit:%d Offset:%d]. Found %d inbound links.\n</thought>\n", 
		args.CalleeName, limit, args.Offset, len(results))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleGotoDefinition(ctx context.Context, req *mcp.CallToolRequest, args DefinitionParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing filePath"}},
			IsError: true,
		}, nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("LSP client error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	locs, err := client.Definition(ctx, lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + path},
			Position: lsp.Position{
				Line:      args.Line - 1,
				Character: args.Character - 1,
			},
		},
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Definition lookup failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	thought := fmt.Sprintf("<thought>\nResolved definition for symbol at %s:%d:%d via LSP.\n</thought>\n", args.FilePath, args.Line, args.Character)
	out, _ := json.Marshal(locs)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleTypeInfo(ctx context.Context, req *mcp.CallToolRequest, args TypeInfoParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing filePath"}},
			IsError: true,
		}, nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("LSP client error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	res, err := client.Hover(ctx, lsp.HoverParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + path},
			Position: lsp.Position{
				Line:      args.Line - 1,
				Character: args.Character - 1,
			},
		},
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Hover lookup failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	thought := fmt.Sprintf("<thought>\nRetrieved type info/hover documentation for symbol at %s:%d:%d.\n</thought>\n", args.FilePath, args.Line, args.Character)
	out, _ := json.Marshal(res)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleImpact(ctx context.Context, req *mcp.CallToolRequest, args ImpactParams) (*mcp.CallToolResult, any, error) {
	// [Sovereignty Upgrade] Route through TruthEngine
	res, err := s.engine.AnalyzeImpact(ctx, args.SymbolName, args.FilePath, args.Verbose, &mcpMessenger{req: req})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to analyze impact: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	out, err := json.Marshal(res)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal impact result: %v", err)}},
			IsError: true,
		}, nil, nil
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
		}, nil, nil
	}
	out, _ := json.Marshal(results)

	thought := fmt.Sprintf("<thought>\nRisk Analysis: Identifying high-risk symbols (high centrality and fragility). Found %d targets (limit: %d).\n</thought>\n", len(results), limit)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) handleStructuralSearch(ctx context.Context, req *mcp.CallToolRequest, args StructuralSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" || args.Ext == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing pattern or ext"}},
			IsError: true,
		}, nil, nil
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = "."
	}

	path, err := utils.ValidatePath(searchPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	results, err := engine.StructuralSearch(ctx, path, args.Pattern, args.Ext)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Structural search failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	count := len(results)
	if count > 500 {
		results = results[:500]
	}

	out, err := json.Marshal(results)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal structural search results: %v", err)}},
			IsError: true,
		}, nil, nil
	}
	
	thought := fmt.Sprintf("<thought>\nExecuted structural search for pattern '%s' in '%s'. Found %d matches (truncated to 500).\n</thought>\n", args.Pattern, path, count)
	
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
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
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing query"}},
			IsError: true,
		}, nil, nil
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
		}, nil, nil
	}
	out, err := json.Marshal(res)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal hybrid search results: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	thought := fmt.Sprintf("<thought>\nExecuted hybrid search for '%s'. Found %d AST symbols and %d Engram insights (limit: %d, offset: %d).\n</thought>\n", args.Query, len(res.Symbols), len(res.Insights), limit, args.Offset)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
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
			sb.WriteString("\nPlease ensure these are documented with high fidelity.")
			systemPrompt = sb.String()
		}
	}

	// 1. Sampling Request (Self-Summarization Loop)
	samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: "Please provide the high-density technical summary for compaction."},
			},
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

type SaveAnchorParams struct {
	Summary string `json:"summary"`
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
	cmd := exec.CommandContext(ctx, "engram", "save", "--title", title, "--type", "session_summary", "--project", project, "--", engramContent)
	if err := cmd.Run(); err != nil {
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

type healerMessenger struct {
	req       *mcp.CallToolRequest
	engramCtx string
}

func (m *healerMessenger) Ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	sysPrompt := SelfHealSystemPrompt
	if m.engramCtx != "" {
		sysPrompt += "\n\nHISTORICAL FIXES:\n" + m.engramCtx
	}
	res, err := m.req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: sysPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: userPrompt}},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return "", err
	}
	txt, ok := res.Content.(*mcp.TextContent)
	if !ok {
		return "", fmt.Errorf("unexpected sampling response type")
	}
	return txt.Text, nil
}

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
	engramCtx := fetchEngramContext("bugfix " + searchQuery)

	// Delegate to TruthEngine
	res, err := s.engine.Fix(ctx, args.ErrorLog, &healerMessenger{req: req, engramCtx: engramCtx})
	if err != nil {
		return nil, nil, fmt.Errorf("self-heal failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
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
	res, err := s.engine.Propagate(ctx, args.SymbolName, args.Transformation, &mcpMessenger{req: req})
	if err != nil {
		return nil, nil, fmt.Errorf("propagation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res},
		},
	}, nil, nil
}

func (s *Server) handleEvolve(ctx context.Context, req *mcp.CallToolRequest, args EvolveParams) (*mcp.CallToolResult, any, error) {
	if args.Proposal == "" {
		return nil, nil, fmt.Errorf("missing proposal")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Sampling: Request Genome Mutation
	samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: GEPSystemPrompt,
		Messages: []*mcp.SamplingMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: args.Proposal},
			},
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

	// 2. Parse JSON Mutations (Robust Extraction)
	var mutations []struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	rawJSON := utils.ExtractJSON(txt.Text)
	if err := json.Unmarshal([]byte(rawJSON), &mutations); err != nil {
		return nil, nil, fmt.Errorf("failed to parse mutation JSON: %w\nRaw: %s", err, txt.Text)
	}

	// [Strike 3 Redemption] Core Armor Protection
	for _, m := range mutations {
		if !args.Force && strings.Contains(m.File, "internal/mcp/handlers.go") {
			return nil, nil, fmt.Errorf("SOVEREIGNTY VIOLATION: Mutation attempts to modify GEP core logic in '%s'. Use 'force:true' if this is an intended self-lobotomy.", m.File)
		}
	}

	// 3. Atomic Snapshots & Application
	backups := make(map[string][]byte)
	rollback := func() {
		for f, b := range backups {
			if b == nil { os.Remove(f) } else { os.WriteFile(f, b, 0644) }
		}
	}

	for _, m := range mutations {
		cleanPath, err := utils.ValidatePath(m.File)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid path in mutation: %s - %w", m.File, err)
		}

		// Read original for backup
		original, err := os.ReadFile(cleanPath)
		if err != nil {
			backups[cleanPath] = nil
		} else {
			backups[cleanPath] = original
		}

		// Apply mutation
		if err := os.WriteFile(cleanPath, []byte(m.Content), 0644); err != nil {
			rollback()
			return nil, nil, fmt.Errorf("failed to apply mutation to %s: %w", cleanPath, err)
		}
	}

	// 4. [Strike 2 Redemption] Ouroboros Verification (Build + Smoke Test + Unit Tests)
	// A. Compilation
	buildCmd := exec.CommandContext(ctx, "just", "build")
	if buildOut, err := buildCmd.CombinedOutput(); err != nil {
		rollback()
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("❌ Evolution failed: Compilation Error\n\n%s", string(buildOut))},
			},
		}, nil, nil
	}

	// B. [NEW] Runtime Smoke Test (Detect start-up panics)
	smokeCmd := exec.CommandContext(ctx, "./bin/scouter", "--version")
	if smokeOut, err := smokeCmd.CombinedOutput(); err != nil {
		rollback()
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("❌ Evolution failed: Runtime Smoke Test (Possible startup panic)\n\n%s", string(smokeOut))},
			},
		}, nil, nil
	}

	// C. Unit Tests
	testCmd := exec.CommandContext(ctx, "go", "test", "./...")
	if testOut, err := testCmd.CombinedOutput(); err != nil {
		rollback()
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("❌ Evolution failed: Test Failures\n\n%s", string(testOut))},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Evolution Successful. Applied mutations to " + fmt.Sprint(len(mutations)) + " files."},
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

var execCommand = exec.Command

func fetchEngramContext(query string) string {
	cmd := execCommand("engram", "search", query, "--limit", "3")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	res := string(out)
	if len(res) > 1000 {
		return res[:1000] + "\n...[truncated]"
	}
	return res
}

func (s *Server) handleJudge(ctx context.Context, req *mcp.CallToolRequest, args JudgeParams) (*mcp.CallToolResult, any, error) {
	engramCtx := fetchEngramContext("architecture decisions ADR " + args.Proposal)
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

// mcpMessenger adapts MCP Sampling to TruthEngine's Messenger interface.
type mcpMessenger struct {
	req *mcp.CallToolRequest
}

func (m *mcpMessenger) Ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	res, err := m.req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: userPrompt}},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return "", err
	}
	txt, ok := res.Content.(*mcp.TextContent)
	if !ok {
		return "", fmt.Errorf("unexpected sampling response type")
	}
	return txt.Text, nil
}
