package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"

	"bytes"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
)

type IndexParams struct {
	FilePath string `json:"filePath" jsonschema:"REQUIRED. The absolute or relative path to the file or directory to index"`
}

type MapParams struct {
	Path string `json:"path" jsonschema:"REQUIRED. The absolute or relative path to the file or directory to map (skeleton only)"`
}

type SearchParams struct {
	Query  string `json:"query" jsonschema:"REQUIRED. The search query (supports semantic or text search)"`
	Type   string `json:"type,omitempty" jsonschema:"Optional: Filter by symbol type (e.g., function, method, struct)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Optional: Max results to return (default: 50, max: 100)"`
	Offset int    `json:"offset,omitempty" jsonschema:"Optional: Number of results to skip for pagination"`
	Format string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
}

type ReadParams struct {
	FilePath string `json:"filePath" jsonschema:"REQUIRED. Path to the file to read"`
	Pointer  string `json:"pointer" jsonschema:"REQUIRED. AST pointer or fragment identifier to read (e.g., 'symbol:MyFunc')"`
}

type CallersParams struct {
	CalleeName string `json:"calleeName" jsonschema:"REQUIRED. The name of the function or method to find callers for"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Optional: Max results to return (default: 50, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Optional: Number of results to skip for pagination"`
	Format     string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
}

type DefinitionParams struct {
	FilePath  string `json:"filePath" jsonschema:"REQUIRED. Path to the file containing the symbol"`
	Line      int    `json:"line" jsonschema:"REQUIRED. 1-based line number"`
	Character int    `json:"character" jsonschema:"REQUIRED. 1-based character position"`
}

type TypeInfoParams struct {
	FilePath  string `json:"filePath" jsonschema:"REQUIRED. Path to the file containing the symbol"`
	Line      int    `json:"line" jsonschema:"REQUIRED. 1-based line number"`
	Character int    `json:"character" jsonschema:"REQUIRED. 1-based character position"`
}

type StructuralSearchParams struct {
	Pattern      string `json:"pattern,omitempty" jsonschema:"Optional: The structural search pattern (supports $VAR and $$$ wildcards)"`
	TargetSymbol string `json:"targetSymbol,omitempty" jsonschema:"Optional: An existing symbol to use as the template pattern (Find Logical Twins)"`
	Ext          string `json:"ext,omitempty" jsonschema:"Optional: The file extension to search in (e.g., '.go', '.ts')"`
	Path         string `json:"path,omitempty" jsonschema:"Optional: Root path for the search (defaults to '.')"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Optional: Max results to return (default: 50, max: 100)"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Optional: Number of results to skip for pagination"`
}

type DiagnoseParams struct {
	ErrorLog string `json:"errorLog" jsonschema:"REQUIRED. The error log output containing the file and line number of the failure"`
}

type NeighborhoodParams struct {
	FilePath string `json:"filePath" jsonschema:"REQUIRED. The absolute or relative path to the file to extract the neighborhood from"`
}

func (s *Server) handleMap(ctx context.Context, req *mcp.CallToolRequest, args MapParams) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return s.presenter.FormatError(fmt.Errorf("missing path")), nil, nil
	}

	path, err := utils.ValidatePath(args.Path)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	results, err := s.store.GetSymbolsByPathPrefix(ctx, path)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Map execution failed: %v", err)), nil, nil
	}

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("No symbols found for path: %s", path)}},
		}, nil, nil
	}

	// Group by file
	fileMap := make(map[string][]string)
	for _, sym := range results {
		fileMap[sym.Path] = append(fileMap[sym.Path], fmt.Sprintf("[%s] %s (Line: %d)", sym.Type, sym.Signature, sym.StartLine))
	}

	var buf bytes.Buffer
	for file, syms := range fileMap {
		buf.WriteString(fmt.Sprintf("=== %s ===\n", file))
		for _, sym := range syms {
			buf.WriteString(sym + "\n")
		}
		buf.WriteString("\n")
	}

	thought := fmt.Sprintf("Sovereign Map: Extracted skeleton for %s. Found %d symbols across %d files. Function bodies were excluded to save tokens.", args.Path, len(results), len(fileMap))

	return s.presenter.FormatTextResult(thought, buf.String()), nil, nil
}

func (s *Server) handleIndex(ctx context.Context, req *mcp.CallToolRequest, args IndexParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return s.presenter.FormatError(fmt.Errorf("missing filePath")), nil, nil
	}

	thought := fmt.Sprintf("Indexing AST symbols for path: %s. This will update the global call graph and enable precise impact analysis.", args.FilePath)

	if err := s.indexer.Index(ctx, args.FilePath); err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Indexing failed: %v", err)}},
				IsError: true,
			},
			nil, nil
	}

	return s.presenter.FormatTextResult(thought, fmt.Sprintf("✅ Indexed %s and updated global call graph", args.FilePath)), nil, nil
}

func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // Sovereign Limit
	}

	searchRes, err := s.discovery.HybridSearch(ctx, args.Query, limit, args.Offset)
	if err != nil {
		return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search execution failed: %v", err)}},
				IsError: true,
			},
			nil, nil
	}

	thought := fmt.Sprintf("Sovereign Search: Querying AST+Engram for '%s' (%s). Pagination: [Limit:%d Offset:%d]. Found %d matches & %d insights.",
		args.Query, args.Type, limit, args.Offset, len(searchRes.Symbols), len(searchRes.Insights))

	resultPayload, err := s.presenter.FormatResult(thought, searchRes)
	return resultPayload, nil, err
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

	thought := fmt.Sprintf("Reading fragment from %s (pointer: %s). Resolved to range %d:%d.",
		args.FilePath, args.Pointer, rng.Start, rng.End)
	return s.presenter.FormatTextResult(thought, content), nil, nil
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

	for i := range results {
		callerSymbols, err := s.store.GetSymbolsByNameInFile(ctx, results[i].CallerName, results[i].Path)
		if err == nil && len(callerSymbols) > 0 {
			sym := callerSymbols[0]
			if bodyExtracted, err := utils.ExtractLines(sym.Path, sym.StartLine, sym.EndLine); err == nil {
				results[i].Body = strings.TrimSpace(bodyExtracted)
			}
		}
	}

	thought := fmt.Sprintf("Call Graph Analysis: Finding all callers of '%s'. Pagination: [Limit:%d Offset:%d]. Found %d callers.",
		args.CalleeName, limit, args.Offset, len(results))

	resultPayload, err := s.presenter.FormatResult(thought, results)
	return resultPayload, nil, err
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

	thought := fmt.Sprintf("LSP Navigation: Finding definition at %s:%d:%d.", args.FilePath, args.Line, args.Character)

	resultPayload, formatErr := s.presenter.FormatResult(thought, result)
	return resultPayload, nil, formatErr
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
	thought := fmt.Sprintf("LSP Inspection: Extracting type information/hover docs at %s:%d:%d.", args.FilePath, args.Line, args.Character)

	resultPayload, formatErr := s.presenter.FormatResult(thought, result)
	return resultPayload, nil, formatErr
}

func (s *Server) handleStructuralSearch(ctx context.Context, req *mcp.CallToolRequest, args StructuralSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" && args.TargetSymbol == "" {
		return s.presenter.FormatError(fmt.Errorf("missing pattern or targetSymbol")), nil, nil
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = "."
	}

	path, err := utils.ValidatePath(searchPath)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	if args.TargetSymbol != "" {
		results, err := s.discovery.FindLogicalTwins(ctx, args.TargetSymbol, path)
		if err != nil {
			return s.presenter.FormatError(fmt.Errorf("Failed to find logical twins: %v", err)), nil, nil
		}
		total := len(results)
		if offset >= total {
			results = nil
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			results = results[offset:end]
		}

		thought := fmt.Sprintf("Structural Analysis: Identifying symbols with identical logical signatures to '%s' in '%s'. Pagination: [Limit:%d Offset:%d]. Found %d matches (Total: %d).", args.TargetSymbol, path, limit, offset, len(results), total)

		resultPayload, formatErr := s.presenter.FormatResult(thought, results)
		return resultPayload, nil, formatErr
	}

	action, ok := filter.GetAction("ast_grep")
	if !ok {
		return s.presenter.FormatError(fmt.Errorf("ast_grep action not found in registry")), nil, nil
	}

	// Prepare parameters for the ast_grep action
	params := map[string]any{
		"pattern": args.Pattern,
		"path":    path,
	}

	// Map extension to language if possible, or let ast-grep auto-detect based on extension
	// ast-grep usually auto-detects if we pass a path, but we can enforce it if needed.
	// We'll pass the extension as a hint if we want, but ast-grep handles directory scanning well.
	// Actually, if we pass a directory, ast-grep will scan all supported files.
	// If the user provided an extension, we might need to filter the results or pass it to ast-grep.
	// ast-grep doesn't have a direct --ext flag, it uses --lang.
	// Let's map common extensions to languages.
	lang := ""
	switch args.Ext {
	case ".go", "go":
		lang = "go"
	case ".ts", "ts":
		lang = "typescript"
	case ".js", "js":
		lang = "javascript"
	case ".rs", "rs":
		lang = "rust"
	case ".py", "py":
		lang = "python"
	}
	if lang != "" {
		params["lang"] = lang
	}

	// Execute the filter
	input := filter.ActionResult{Metadata: map[string]any{"path": path}}
	res, err := action(ctx, input, params)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Structural search failed: %v", err)), nil, nil
	}

	// The result lines are JSON strings from ast-grep's --json=stream
	// We need to parse them back into StructuralMatch objects to maintain API compatibility
	var results []engine.StructuralMatch
	for _, line := range res.Lines {
		var match filter.AstGrepMatch
		if err := json.Unmarshal([]byte(line), &match); err == nil {
			matchPath := match.File
			if matchPath == "" {
				matchPath = path
			}
			results = append(results, engine.StructuralMatch{
				Path:      matchPath,
				StartLine: match.Range.Start.Line,
				EndLine:   match.Range.End.Line,
				Content:   match.Text,
			})
		}
	}

	total := len(results)

	if offset >= total {
		results = []engine.StructuralMatch{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		results = results[offset:end]
	}

	thought := fmt.Sprintf("Executed structural search for pattern '%s' in '%s'. Pagination: [Limit:%d Offset:%d]. Found %d matches (Total: %d).",
		args.Pattern, path, limit, offset, len(results), total)

	resultPayload, formatErr := s.presenter.FormatResult(thought, results)
	return resultPayload, nil, formatErr
}

func (s *Server) handleDiagnose(ctx context.Context, req *mcp.CallToolRequest, args DiagnoseParams) (*mcp.CallToolResult, any, error) {
	if args.ErrorLog == "" {
		return s.presenter.FormatError(fmt.Errorf("missing errorLog")), nil, nil
	}

	hudStruct, err := s.healer.DiagnoseHUD(ctx, args.ErrorLog)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Diagnose failed: %v", err)), nil, nil
	}

	thought := "Executing Diagnostic Vision (Fase 8). Fused Git Provenance, X-Ray AST, Radar Impact, and Thermal Similarity into HUD."

	resultPayload, formatErr := s.presenter.FormatResult(thought, []engine.DiagnosticHUD{*hudStruct})

	return resultPayload, nil, formatErr
}

func (s *Server) handleNeighborhood(ctx context.Context, req *mcp.CallToolRequest, args NeighborhoodParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return s.presenter.FormatError(fmt.Errorf("missing filePath")), nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	neighborhood, err := s.discovery.GetNeighborhood(ctx, path)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Failed to get neighborhood: %v", err)), nil, nil
	}

	thought := fmt.Sprintf("ZON Neighborhood: Extracted 1-hop structural context for %s.", args.FilePath)

	return s.presenter.FormatTextResult(thought, neighborhood), nil, nil
}
