package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/Rogercode97/scouter/internal/display"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"bytes"
	"strings"
)

type InspectArgs struct {
	Mode     string        `json:"mode" jsonschema:"The mode of inspection (skeleton|read|callers|definition|type_info)"`
	FilePath string        `json:"filePath" jsonschema:"The absolute or relative path to the file or directory to inspect"`
	Symbol   string        `json:"symbol,omitempty" jsonschema:"Optional: The symbol to inspect (for read/definition/type_info/callers)"`
	Position *lsp.Position `json:"position,omitempty" jsonschema:"Optional: The LSP position to inspect (mutually exclusive with symbol)"`
}

type SearchArgs struct {
	Mode         string `json:"mode" jsonschema:"The mode of search (text|structural|index)"`
	Query        string `json:"query,omitempty" jsonschema:"The search query (required for text/structural)"`
	Directory    string `json:"directory,omitempty" jsonschema:"The directory to index (required for index)"`
	Type         string `json:"type,omitempty" jsonschema:"Optional: Filter by symbol type"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max results to return"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
	Format       string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
	TargetSymbol string `json:"targetSymbol,omitempty" jsonschema:"Optional: Target symbol for structural search"`
	Ext          string `json:"ext,omitempty" jsonschema:"Optional: File extension for structural search"`
	Path         string `json:"path,omitempty" jsonschema:"Optional: Root path for structural search"`
}

type IndexParams struct {
	FilePath string `json:"filePath" jsonschema:"The absolute or relative path to the file or directory to index"`
}

type MapParams struct {
	Path string `json:"path" jsonschema:"The absolute or relative path to the file or directory to map (skeleton only)"`
}

type SearchParams struct {
	Query  string `json:"query" jsonschema:"The search query (supports semantic or text search)"`
	Type   string `json:"type,omitempty" jsonschema:"Optional: Filter by symbol type (e.g., function, method, struct)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
	Format string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
}

type ReadParams struct {
	FilePath string `json:"filePath" jsonschema:"Path to the file to read"`
	Pointer  string `json:"pointer" jsonschema:"AST pointer or fragment identifier to read (e.g., 'symbol:MyFunc')"`
}

type CallersParams struct {
	CalleeName string `json:"calleeName" jsonschema:"The name of the function or method to find callers for"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
	Format     string `json:"format,omitempty" jsonschema:"Optional: Response format ('text' or 'hakai')"`
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
	Pattern      string `json:"pattern,omitempty" jsonschema:"Optional: The structural search pattern (supports $VAR and $$$ wildcards)"`
	TargetSymbol string `json:"targetSymbol,omitempty" jsonschema:"Optional: An existing symbol to use as the template pattern (Find Logical Twins)"`
	Ext          string `json:"ext,omitempty" jsonschema:"Optional: The file extension to search in (e.g., '.go', '.ts')"`
	Path         string `json:"path,omitempty" jsonschema:"Optional: Root path for the search (defaults to '.')"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max results to return (default: 50, max: 100)"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
}

type DiagnoseParams struct {
	ErrorLog string `json:"errorLog" jsonschema:"The error log output containing the file and line number of the failure"`
}

func (s *Server) HandleInspect(ctx context.Context, req *mcp.CallToolRequest, args InspectArgs) (*mcp.CallToolResult, any, error) {
	switch args.Mode {
	case "skeleton":
		return s.executeMap(ctx, req, MapParams{Path: args.FilePath})
	case "read":
		if args.Symbol == "" && args.Position == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "symbol or position required for read mode"}}, IsError: true}, nil, nil
		}
		pointer := args.Symbol
		if pointer == "" && args.Position != nil {
			pointer = fmt.Sprintf("position:%d:%d", args.Position.Line, args.Position.Character)
		}
		return s.executeRead(ctx, req, ReadParams{FilePath: args.FilePath, Pointer: pointer})
	case "callers":
		if args.Symbol == "" && args.Position == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "symbol or position required for callers mode"}}, IsError: true}, nil, nil
		}
		return s.executeCallers(ctx, req, CallersParams{CalleeName: args.Symbol})
	case "definition":
		if args.Position == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "position required for definition mode"}}, IsError: true}, nil, nil
		}
		return s.executeDefinition(ctx, req, DefinitionParams{FilePath: args.FilePath, Line: args.Position.Line, Character: args.Position.Character})
	case "type_info":
		if args.Position == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "position required for type_info mode"}}, IsError: true}, nil, nil
		}
		return s.executeTypeInfo(ctx, req, TypeInfoParams{FilePath: args.FilePath, Line: args.Position.Line, Character: args.Position.Character})
	default:
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invalid mode for inspect"}}, IsError: true}, nil, nil
	}
}

func (s *Server) HandleSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, any, error) {
	switch args.Mode {
	case "text":
		if args.Query == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "query required for text search"}}, IsError: true}, nil, nil
		}
		return s.executeSearch(ctx, req, SearchParams{Query: args.Query, Type: args.Type, Limit: args.Limit, Offset: args.Offset, Format: args.Format})
	case "structural":
		if args.Query == "" && args.TargetSymbol == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "query or targetSymbol required for structural search"}}, IsError: true}, nil, nil
		}
		return s.executeStructuralSearch(ctx, req, StructuralSearchParams{Pattern: args.Query, TargetSymbol: args.TargetSymbol, Ext: args.Ext, Path: args.Path, Limit: args.Limit, Offset: args.Offset})
	case "index":
		if args.Directory == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "directory required for index mode"}}, IsError: true}, nil, nil
		}
		return s.executeIndex(ctx, req, IndexParams{FilePath: args.Directory})
	default:
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invalid mode for search"}}, IsError: true}, nil, nil
	}
}

func (s *Server) executeMap(ctx context.Context, req *mcp.CallToolRequest, args MapParams) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing path"}},
			IsError: true,
		}, nil, nil
	}

	path, err := utils.ValidatePath(args.Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}

	results, err := s.store.GetSymbolsByPathPrefix(ctx, path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Map execution failed: %v", err)}},
			IsError: true,
		}, nil, nil
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

	thought := fmt.Sprintf("<thought>\nSovereign Map: Extracted skeleton for %s. Found %d symbols across %d files. Function bodies were excluded to save tokens.\n</thought>\n", args.Path, len(results), len(fileMap))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + buf.String()},
		},
	}, nil, nil
}

func (s *Server) executeIndex(ctx context.Context, req *mcp.CallToolRequest, args IndexParams) (*mcp.CallToolResult, any, error) {
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

func (s *Server) executeSearch(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // Sovereign Limit
	}

	searchRes, err := s.engine.HybridSearch(ctx, args.Query, limit, args.Offset)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search execution failed: %v", err)}},
			IsError: true,
		},
		nil, nil
	}

	var outStr string
	useHakai := args.Format == "hakai" || (args.Format == "" && len(searchRes.Symbols) > 20)

	if useHakai {
		var buf bytes.Buffer
		sw := display.NewSovereignWrapper(&buf)
		// Initialize ACCP with centralized thresholds
		sw.SetACCP(display.NewACCP(display.DefaultThresholdWarm, display.DefaultThresholdCold))
		sw.WriteHeader()
		for _, sym := range searchRes.Symbols {
			sSym := store.Symbol{
				Name: sym.Name,
				Type: sym.Type,
				Signature: sym.Signature,
				Path: sym.Path,
				StartLine: sym.StartLine,
				EndLine: sym.EndLine,
				Doc: sym.Doc,
			}
			sw.EmitSymbol(sSym)
		}
		sw.Flush()
		outStr = buf.String()
	} else {
		if args.Format == "zon" || args.Format == "" {
			zonSyms, _ := engine.EncodeZON(searchRes.Symbols)
			zonInsights, _ := engine.EncodeZON(searchRes.Insights)
			outStr = zonSyms + "\nInsights:\n" + zonInsights
			if len(outStr) > 4096 {
				outStr = outStr[:4000] + "\n... [TRUNCATED BY SOVEREIGN GUARD (4KB LIMIT)]"
			}
		} else {
			out, _ := json.Marshal(searchRes)
			if string(out) == "null" {
				out = []byte("{}")
			}
			outStr = string(out)
		}
	}

	thought := fmt.Sprintf("<thought>\nSovereign Search: Querying AST+Engram for '%s' (%s). Pagination: [Limit:%d Offset:%d]. Found %d matches & %d insights. Format: %s.\n</thought>\n",
		args.Query, args.Type, limit, args.Offset, len(searchRes.Symbols), len(searchRes.Insights), map[bool]string{true: "hakai", false: "json"}[useHakai])

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + outStr},
		},
	}, nil, nil
}

func (s *Server) executeRead(ctx context.Context, req *mcp.CallToolRequest, args ReadParams) (*mcp.CallToolResult, any, error) {
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

func (s *Server) executeCallers(ctx context.Context, req *mcp.CallToolRequest, args CallersParams) (*mcp.CallToolResult, any, error) {
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

	var outStr string
	useHakai := args.Format == "hakai" || (args.Format == "" && len(results) > 20)

	if useHakai {
		var buf bytes.Buffer
		sw := display.NewSovereignWrapper(&buf)
		// Initialize ACCP with centralized thresholds
		sw.SetACCP(display.NewACCP(display.DefaultThresholdWarm, display.DefaultThresholdCold))
		sw.WriteHeader()
		for _, call := range results {
			sw.EmitCall(call)
		}
		sw.Flush()
		outStr = buf.String()
	} else {
		if args.Format == "zon" || args.Format == "" {
			outStr, _ = engine.EncodeZON(results)
			if len(outStr) > 4096 {
				outStr = outStr[:4000] + "\n... [TRUNCATED BY SOVEREIGN GUARD (4KB LIMIT)]"
			}
		} else {
			out, _ := json.Marshal(results)
			if string(out) == "null" {
				out = []byte("[]")
			}
			outStr = string(out)
		}
	}

	thought := fmt.Sprintf("<thought>\nCall Graph Analysis: Finding all callers of '%s'. Pagination: [Limit:%d Offset:%d]. Found %d callers. Format: %s.\n</thought>\n",
		args.CalleeName, limit, args.Offset, len(results), map[bool]string{true: "hakai", false: "json"}[useHakai])

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + outStr},
		},
	}, nil, nil
}

func (s *Server) executeDefinition(ctx context.Context, req *mcp.CallToolRequest, args DefinitionParams) (*mcp.CallToolResult, any, error) {
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
	if string(out) == "null" {
		out = []byte("[]")
	}
	thought := fmt.Sprintf("<thought>\nLSP Navigation: Finding definition at %s:%d:%d.\n</thought>\n", args.FilePath, args.Line, args.Character)

	return &mcp.CallToolResult{
	        Content: []mcp.Content{
	                &mcp.TextContent{Text: thought + string(out)},
	        },
	}, nil, nil
	}

	func (s *Server) executeTypeInfo(ctx context.Context, req *mcp.CallToolRequest, args TypeInfoParams) (*mcp.CallToolResult, any, error) {
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
	if string(out) == "null" {
		out = []byte("[]")
	}
	thought := fmt.Sprintf("<thought>\nLSP Inspection: Extracting type information/hover docs at %s:%d:%d.\n</thought>\n", args.FilePath, args.Line, args.Character)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + string(out)},
		},
	}, nil, nil
}

func (s *Server) executeStructuralSearch(ctx context.Context, req *mcp.CallToolRequest, args StructuralSearchParams) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" && args.TargetSymbol == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing pattern or targetSymbol"}},
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

	limit := args.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	if args.TargetSymbol != "" {
		results, err := s.engine.FindLogicalTwins(ctx, args.TargetSymbol, path)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to find logical twins: %v", err)}},
				IsError: true,
			}, nil, nil
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
		
		outStr, _ := engine.EncodeZON(results)
		if len(outStr) > 4096 {
			outStr = outStr[:4000] + "\n... [TRUNCATED BY SOVEREIGN GUARD (4KB LIMIT)]"
		}
		thought := fmt.Sprintf("<thought>\nStructural Analysis: Identifying symbols with identical logical signatures to '%s' in '%s'. Pagination: [Limit:%d Offset:%d]. Found %d matches (Total: %d).\n</thought>\n", args.TargetSymbol, path, limit, offset, len(results), total)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: thought + outStr},
			},
		}, nil, nil
	}

	action, ok := filter.GetAction("ast_grep")
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ast_grep action not found in registry"}},
			IsError: true,
		}, nil, nil
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
	case ".go", "go": lang = "go"
	case ".ts", "ts": lang = "typescript"
	case ".js", "js": lang = "javascript"
	case ".rs", "rs": lang = "rust"
	case ".py", "py": lang = "python"
	}
	if lang != "" {
		params["lang"] = lang
	}

	// Execute the filter
	input := filter.ActionResult{Metadata: map[string]any{"path": path}}
	res, err := action(ctx, input, params)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Structural search failed: %v", err)}},
			IsError: true,
		}, nil, nil
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

	var outStr string
	outStr, _ = engine.EncodeZON(results)
	if len(outStr) > 4096 {
		outStr = outStr[:4000] + "\n... [TRUNCATED BY SOVEREIGN GUARD (4KB LIMIT)]"
	}

	thought := fmt.Sprintf("<thought>\nExecuted structural search for pattern '%s' in '%s'. Pagination: [Limit:%d Offset:%d]. Found %d matches (Total: %d).\n</thought>\n",
		args.Pattern, path, limit, offset, len(results), total)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + outStr},
		},
	}, nil, nil
}

func (s *Server) handleDiagnose(ctx context.Context, req *mcp.CallToolRequest, args DiagnoseParams) (*mcp.CallToolResult, any, error) {
	if args.ErrorLog == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "missing errorLog"}},
			IsError: true,
		}, nil, nil
	}

	hudOutput, err := s.engine.Healer().DiagnoseHUD(ctx, args.ErrorLog)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Diagnose failed: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	thought := "<thought>\nExecuting Diagnostic Vision (Fase 8). Fused Git Provenance, X-Ray AST, Radar Impact, and Thermal Similarity into HUD.\n</thought>\n"

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + hudOutput},
		},
	}, nil, nil
}
