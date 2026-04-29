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
	"github.com/Rogercode97/scouter/internal/store"
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
		return nil, nil, fmt.Errorf("missing filePath")
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return nil, nil, err
	}

	pointers, calls, err := engine.ParseFile(ctx, path, s.lspMgr)
	if err != nil {
		return nil, nil, err
	}

	// Calculate hash for file index integrity
	hash, _ := utils.CalculateHash(path)
	fi, _ := os.Stat(path)
	mtime := int64(0)
	if fi != nil {
		mtime = fi.ModTime().Unix()
	}

	// Persist to store (Sovereignty Mandate: Done is validated)
	err = s.store.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
		tx.SaveFileIndex(ctx, &store.FileIndex{
			Path:    path,
			Mtime:   mtime,
			Hash:    hash,
			ASTJSON: "{}", // Compact context optimization
			Project: utils.GetRepoName(ctx),
		})
		tx.ClearSymbols(ctx, path)
		tx.ClearCalls(ctx, path)

		for _, p := range pointers {
			err := tx.SaveSymbol(ctx, &store.Symbol{
				Name:      p.Name,
				Type:      p.Type,
				Signature: p.Signature,
				Doc:       p.Doc,
				Path:      path,
				StartByte: p.Range.Start,
				EndByte:   p.Range.End,
				StartLine: p.StartLine,
				StartCol:  p.StartCol,
				EndLine:   p.EndLine,
			})
			if err != nil {
				return err
			}
		}

		for _, c := range calls {
			err := tx.SaveCall(ctx, store.Call{
				CallerName: c.CallerName,
				CalleeName: c.CalleeName,
				CalleePath: c.CalleePath,
				LinkType:   c.LinkType,
				Path:       path,
				Line:       c.Line,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to save index: %w", err)
	}

	// DIVINE REDEMPTION: Post-indexing resolution (Interfaces & Centrality)
	// This ensures the Global Call Graph is immediately updated after an explicit index call.
	_ = engine.LinkInterfaces(ctx, s.store, s.lspMgr)
	_ = s.store.ResolveCentrality(ctx)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("✅ Indexed %s: %d symbols, %d calls", args.FilePath, len(pointers), len(calls))},
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

func (s *Server) handleGotoDefinition(ctx context.Context, req *mcp.CallToolRequest, args DefinitionParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return nil, nil, fmt.Errorf("missing filePath")
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return nil, nil, err
	}

	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	out, _ := json.Marshal(locs)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleTypeInfo(ctx context.Context, req *mcp.CallToolRequest, args TypeInfoParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return nil, nil, fmt.Errorf("missing filePath")
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return nil, nil, err
	}

	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	out, _ := json.Marshal(res)
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

	// 4. [Divine Synergy] Progressive Disclosure (Context OS)
	if !args.Verbose {
		summary := map[string]any{
			"symbol":      res.Target.Symbol,
			"file":        res.Target.File,
			"risk_score":  res.Target.RiskScore,
			"risk_level":  res.RiskLevel,
			"callers":     len(res.Callers),
			"instruction": "For full Mermaid graph and callers list, use 'verbose: true'.",
		}
		summaryJSON, _ := json.Marshal(summary)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(summaryJSON)}}}, nil, nil
	}

	out, err := json.Marshal(res)
	if err != nil {
		return nil, nil, err
	}

	// 5. [Divine Synergy] Sampling Oracle
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

	res, err := s.search.HybridSearch(ctx, args.Query)
	if err != nil {
		return nil, nil, err
	}
	out, _ := json.Marshal(res)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}

func (s *Server) handleCompactContext(ctx context.Context, req *mcp.CallToolRequest, args CompactContextParams) (*mcp.CallToolResult, any, error) {
	// 1. Sampling Request
	samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: CompactContextSystemPrompt,
		Messages: []*mcp.SamplingMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: "Please provide a high-density summary of our current technical state, tasks, and decisions."},
			},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sampling compaction failed: %w", err)
	}

	txt, ok := samplingRes.Content.(*mcp.TextContent)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected sampling response type")
	}

	// 2. Persistence
	scouterDir := ".scouter"
	if err := os.MkdirAll(scouterDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create scouter directory: %w", err)
	}

	anchorPath := filepath.Join(scouterDir, "anchor.md")
	header := fmt.Sprintf("# 🏛️ SCOUTER ANCHOR\n*Compacted on: %s*\n\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(anchorPath, []byte(header+txt.Text), 0644); err != nil {
		return nil, nil, fmt.Errorf("failed to write anchor file: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Context compacted and anchored to: " + anchorPath},
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

func (s *Server) handleSelfHeal(ctx context.Context, req *mcp.CallToolRequest, args SelfHealParams) (*mcp.CallToolResult, any, error) {
	if args.ErrorLog == "" {
		return nil, nil, fmt.Errorf("missing errorLog")
	}

	// [Sovereignty Mandate] Serial execution via Mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// Configure healer bridge to use MCP Sampling
	s.healer.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
		samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
			SystemPrompt: SelfHealSystemPrompt,
			Messages: []*mcp.SamplingMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: prompt},
				},
			},
			MaxTokens: 2048,
		})
		if err != nil {
			return "", err
		}
		txt, ok := samplingRes.Content.(*mcp.TextContent)
		if !ok {
			return "", fmt.Errorf("unexpected fix response type")
		}
		return txt.Text, nil
	}

	// Delegate to Healer Engine
	res, err := s.healer.Fix(ctx, args.ErrorLog)
	if err != nil {
		return nil, nil, fmt.Errorf("self-heal engine failed: %w", err)
	}

	out, _ := json.Marshal(res)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
		},
	}, nil, nil
}

func (s *Server) handleRippleRefactor(ctx context.Context, req *mcp.CallToolRequest, args RippleRefactorParams) (*mcp.CallToolResult, any, error) {
	if args.SymbolName == "" || args.Transformation == "" {
		return nil, nil, fmt.Errorf("missing symbolName or transformation")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// [Sovereignty Upgrade] Bridge MCP Sampling to Ripple Engine
	s.ripple.Transformer = &engine.MCPTransformer{
		DoTransform: func(ctx context.Context, file, symbol, transformation string) (string, error) {
			// Get current file content for context
			content, err := os.ReadFile(file)
			if err != nil {
				return "", fmt.Errorf("failed to read file %s: %w", file, err)
			}

			prompt := fmt.Sprintf("File: %s\nTarget Symbol: %s\nTransformation: %s\n\nSource Code:\n%s", file, symbol, transformation, string(content))
			samplingRes, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
				SystemPrompt: "You are Scouter's Ripple Engine. Apply the requested transformation to the provided source code. Return ONLY the complete modified source code. NO MARKDOWN. NO COMMENTS.",
				Messages: []*mcp.SamplingMessage{
					{Role: "user", Content: &mcp.TextContent{Text: prompt}},
				},
				MaxTokens: 4096,
			})
			if err != nil {
				return "", err
			}
			txt, ok := samplingRes.Content.(*mcp.TextContent)
			if !ok {
				return "", fmt.Errorf("unexpected sampling response type")
			}
			return utils.ExtractCodeBlock(txt.Text), nil
		},
	}

	// Propagate changes via Ripple Engine
	ledger, err := s.ripple.Propagate(ctx, args.SymbolName, args.Transformation, 5)
	if err != nil {
		return nil, nil, fmt.Errorf("propagation failed: %w", err)
	}

	// Transactional Commit
	if err := ledger.Prepare(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to prepare ledger: %w", err)
	}

	if err := ledger.Commit(ctx); err != nil {
		ledger.Rollback(ctx)
		return nil, nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	res := map[string]any{
		"status":        "SUCCESS",
		"affectedFiles": ledger.AffectedFiles(),
	}
	out, _ := json.Marshal(res)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(out)},
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

	results, err := engine.PredictTests(ctx, s.store, diff)
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

func (s *Server) handleJudge(ctx context.Context, req *mcp.CallToolRequest, args JudgeParams) (*mcp.CallToolResult, any, error) {
	prompt := fmt.Sprintf("Architectural Proposal: %s\n\nGit Diff:\n%s", args.Proposal, args.Diff)

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
