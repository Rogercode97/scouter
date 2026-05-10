package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	)



type IndexParams struct {
	FilePath string `json:"filePath"`
}

type SearchParams struct {
	Query  string `json:"query"`
	Type   string `json:"type,omitempty"`
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

type StructuralSearchParams struct {
	Pattern string `json:"pattern"`
	Ext     string `json:"ext"`
	Path    string `json:"path,omitempty"`
}

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
		},
		nil, nil
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
		},
		nil, nil
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
		},
		nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		},
		nil, nil
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
		},
		nil, nil
	}

	content, err := engine.ReadFragment(ctx, path, rng)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		},
		nil, nil
	}

	thought := fmt.Sprintf("<thought>\nReading fragment from %s (pointer: %s). Resolved to range %d:%d.\n</thought>\n",
	        args.FilePath, args.Pointer, rng.Start, rng.End)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + content},
		},
	}, nil, nil
}

func (s *Server) handleCallers(ctx context.Context, req *mcp.CallToolRequest, args CallersParams) (*mcp.CallToolResult, any, error) {
	if args.CalleeName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing calleeName"}},
			IsError: true,
		},
		nil, nil
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
		},
		nil, nil
	}

	out, _ := json.Marshal(results)
	thought := fmt.Sprintf("<thought>\nCall Graph Analysis: Finding all callers of '%s'. Pagination: [Limit:%d Offset:%d]. Found %d callers.\n</thought>\n",
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
		},
		nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		},
		nil, nil
	}

	pos := lsp.Position{
		Line:      args.Line - 1,
		Character: args.Character - 1,
	}

	result, err := s.GotoDefinition(ctx, path, pos)
	if err != nil {
	        return &mcp.CallToolResult{
	                Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Goto definition failed: %v", err)}},
	                IsError: true,
	        },
	        nil, nil
	}

	out, _ := json.Marshal(result)
	thought := fmt.Sprintf("<thought>\nLSP Navigation: Finding definition at %s:%d:%d.\n</thought>\n", args.FilePath, args.Line, args.Character)

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
	        },
	        nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
	        return &mcp.CallToolResult{
	                Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	                IsError: true,
	        },
	        nil, nil
	}

	pos := lsp.Position{
	        Line:      args.Line - 1,
	        Character: args.Character - 1,
	}

	result, err := s.Hover(ctx, path, pos)
	if err != nil {
	        return &mcp.CallToolResult{
	                Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Type info failed: %v", err)}},
	                IsError: true,
	        },
	        nil, nil
	}
	out, _ := json.Marshal(result)
	thought := fmt.Sprintf("<thought>\nLSP Inspection: Extracting type information/hover docs at %s:%d:%d.\n</thought>\n", args.FilePath, args.Line, args.Character)

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
		},
		nil, nil
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
		},
		nil, nil
	}

	results, err := engine.StructuralSearch(ctx, path, args.Pattern, args.Ext)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Structural search failed: %v", err)}},
			IsError: true,
		},
		nil, nil
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
		},
		nil, nil
	}

	thought := fmt.Sprintf("<thought>\nExecuted structural search for pattern '%s' in '%s'. Found %d matches (truncated to 500).\n</thought>\n", args.Pattern, path, count)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}
