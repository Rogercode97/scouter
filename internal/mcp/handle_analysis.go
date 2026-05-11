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
	FilePath string `json:"filePath" jsonschema:"The absolute or relative path to the file or directory to index"`
}

type SearchParams struct {
	Query  string `json:"query" jsonschema:"The search query (supports semantic or text search)"`
	Type   string `json:"type,omitempty" jsonschema:"Optional: Filter by symbol type (e.g., function, method, struct)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
}

type ReadParams struct {
	FilePath string `json:"filePath" jsonschema:"Path to the file to read"`
	Pointer  string `json:"pointer" jsonschema:"AST pointer or fragment identifier to read (e.g., 'symbol:MyFunc')"`
}

type CallersParams struct {
	CalleeName string `json:"calleeName" jsonschema:"The name of the function or method to find callers for"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
}

type DefinitionParams struct {
	FilePath  string `json:"filePath" jsonschema:"Path to the file containing the symbol"`
	Line      int    `json:"line" jsonschema:"1-based line number"`
	Character int    `json:"character" jsonschema:"1-based character position"`
}

type TypeInfoParams struct {
	FilePath  string `json:"filePath" jsonschema:"Path to the file containing the symbol"`
	Line      int    `json:"line" jsonschema:"1-based line number"`
	Character int    `json:"character" jsonschema:"1-based character position"`
}

type StructuralSearchParams struct {
	Pattern string `json:"pattern" jsonschema:"The structural search pattern (supports $VAR and $$$ wildcards)"`
	Ext     string `json:"ext" jsonschema:"The file extension to search in (e.g., '.go', '.ts')"`
	Path    string `json:"path,omitempty" jsonschema:"Optional: Root path for the search (defaults to '.')"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
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

	total := len(results)
	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	if offset >= total {
		results = []engine.StructuralMatch{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		results = results[offset:end]
	}

	out, err := json.Marshal(results)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to marshal structural search results: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	thought := fmt.Sprintf("<thought>\nExecuted structural search for pattern '%s' in '%s'. Pagination: [Limit:%d Offset:%d]. Found %d matches (Total: %d).\n</thought>\n",
		args.Pattern, path, limit, offset, len(results), total)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}
