package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/filter"
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
	results, err := s.store.GetImpact(ctx, args.SymbolName, args.FilePath, maxDepth)
	if err != nil {
		return nil, nil, err
	}
	out, err := json.Marshal(results)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
}

func (s *Server) handleCritical(ctx context.Context, req *mcp.CallToolRequest, args CriticalParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit == 0 {
		limit = 10
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

	res, err := fn(filter.ActionResult{Lines: strings.Split(args.Text, "\n"), Metadata: make(map[string]any)}, map[string]any{"level": level})
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(res.Lines, "\n")},
		},
	}, nil, nil
}
