package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/go-playground/validator/v10"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var version = "2.3.1" // Visual Focus Edition

//go:embed plugins/opencode/scouter.ts
var openCodePluginFS embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "mcp":
		runMCPServer()
	case "setup":
		if len(os.Args) < 3 {
			fmt.Println("Usage: scouter setup <agent> (supported: gemini-cli, opencode)")
			os.Exit(1)
		}
		runSetup(os.Args[2])
	case "version":
		fmt.Printf("scouter %s\n", version)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Scouter — Code analysis engine for AI agents")
	fmt.Println("\nUsage:")
	fmt.Println("  scouter mcp            Start MCP server (stdio transport)")
	fmt.Println("  scouter setup <agent>  Install scouter integration (gemini-cli, opencode)")
	fmt.Println("  scouter version        Show version")
}

func runSetup(agent string) {
	switch agent {
	case "gemini-cli":
		installGeminiCLI()
	case "opencode":
		installOpenCode()
	default:
		fmt.Printf("Error: Agent '%s' not supported yet.\n", agent)
		os.Exit(1)
	}
}

// Glasswall Validation Structs
type IndexRequest struct {
	FilePath string `json:"filePath" validate:"required"`
}

type SearchRequest struct {
	Query string `json:"query" validate:"required,min=1,max=100"`
	Type  string `json:"type" validate:"omitempty,oneof=function class variable method interface"`
}

type ReadRequest struct {
	FilePath string      `json:"filePath" validate:"required"`
	Pointer  types.Range `json:"pointer" validate:"required"`
	Hash     string      `json:"hash" validate:"omitempty,len=64"`
}

type CallersRequest struct {
	CalleeName string `json:"calleeName" validate:"required,min=1"`
}

type ImpactRequest struct {
	SymbolName string `json:"symbolName" validate:"required"`
	FilePath   string `json:"filePath" validate:"omitempty"`
	MaxDepth   int    `json:"maxDepth" validate:"omitempty,min=1,max=10"`
}

type PredictRequest struct {
	MaxDepth int `json:"maxDepth" validate:"omitempty,min=1,max=10"`
}

type CriticalRequest struct {
	Limit int `json:"limit" validate:"omitempty,min=1,max=100"`
}

type VisualizeRequest struct {
	SymbolName string `json:"symbolName" validate:"required,min=1"`
	Depth      int    `json:"depth" validate:"omitempty,min=1,max=3"`
	RiskOnly   bool   `json:"riskOnly"` // Added for Focus Mode
}

type DeadCodeRequest struct {
	IncludeExported bool `json:"includeExported"`
}

type GotoDefinitionRequest struct {
	FilePath  string `json:"filePath" validate:"required"`
	Line      int    `json:"line" validate:"required"`
	Character int    `json:"character" validate:"required"`
}

type TypeInfoRequest struct {
	FilePath  string `json:"filePath" validate:"required"`
	Line      int    `json:"line" validate:"required"`
	Character int    `json:"character" validate:"required"`
}

func runMCPServer() {
	// Create root context with signal notification for Go 1.24+ sovereignty
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Migrate(ctx); err != nil {
		log.Printf("Migration warning: %v", err)
	}

	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer func() {
		log.Println("Closing database connection...")
		db.Close()
	}()

	// Initialize LSP Manager
	lspMgr := lsp.NewManager()
	defer func() {
		log.Println("Closing LSP manager...")
		lspMgr.Close()
	}()

	v := validator.New(validator.WithRequiredStructEnabled())

	s := server.NewMCPServer("scouter", version,
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithInstructions(`Scouter is an AST-based code analysis engine with Semantic Search and LSP Bridge. 
Use 'scouter_index' to understand a file's structure and documentation.
Use 'scouter_search' for intelligent lookups using BM25 ranking.
Use 'scouter_goto_definition' and 'scouter_type_info' for high-precision real-time semantic intelligence.
Use 'scouter_callers' to find all locations where a symbol is invoked.
Use 'scouter_health' to list failed tests and their associated symbols.
Use 'scouter_impact' to calculate the blast radius of changing a symbol.
Use 'scouter_predict' to identify which tests to run based on current local changes (git diff).
Use 'scouter_critical_code' to identify the most central and fragile symbols in the project.
Use 'scouter_visualize' to generate a risk-colored call graph (Mermaid).`),
	)

	// Resource: scouter://status
	statusResource := mcp.NewResource("scouter://status", "Scouter Project Status",
		mcp.WithResourceDescription("Current project indexing statistics"),
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(statusResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		files, symbols, err := db.GetStats(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get stats: %w", err)
		}

		stats := map[string]interface{}{
			"indexed_files":   files,
			"indexed_symbols": symbols,
			"version":         version,
		}
		statsJSON, _ := json.Marshal(stats)

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     string(statsJSON),
			},
		}, nil
	})

	// Tool: scouter_index
	indexTool := mcp.NewTool("scouter_index",
		mcp.WithDescription("Analyze a file to index its structure, calls, and documentation."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The absolute path to the file to index."),
		),
	)

	s.AddTool(indexTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req IndexRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		if err := json.Unmarshal(argsJSON, &req); err != nil {
			return mcpError("Invalid arguments"), nil
		}

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		filePath := req.FilePath
		stats, err := os.Stat(filePath)
		if err != nil {
			return mcpError(fmt.Sprintf("File not found: %v", err)), nil
		}

		currentHash, _ := utils.CalculateHash(filePath)

		cached, err := db.GetFileIndex(ctx, filePath)
		if err == nil && (cached.Mtime == stats.ModTime().UnixNano() || (currentHash != "" && cached.Hash == currentHash)) {
			var cachedSymbols []types.ASTPointer
			if err := json.Unmarshal([]byte(cached.ASTJSON), &cachedSymbols); err == nil {
				responseSymbols := cachedSymbols
				truncated := false
				if len(cachedSymbols) > 500 {
					responseSymbols = cachedSymbols[:500]
					truncated = true
				}
				for i := range responseSymbols {
					responseSymbols[i].Doc = truncateDoc(responseSymbols[i].Doc)
				}
				res := map[string]interface{}{"symbols": responseSymbols, "count": len(cachedSymbols), "truncated": truncated, "cached": true}
				resJSON, _ := json.Marshal(res)
				return mcpJSONResponse(resJSON), nil
			}
		}

		idxResult, calls, err := engine.ParseFile(ctx, filePath, lspMgr)
		if err != nil {
			return mcpError(fmt.Sprintf("Indexing failed: %v", err)), nil
		}

		err = db.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
			astJSON, _ := json.Marshal(idxResult)
			if err := tx.SaveFileIndex(txCtx, &store.FileIndex{
				Path: filePath, Mtime: stats.ModTime().UnixNano(), Hash: currentHash, ASTJSON: string(astJSON),
			}); err != nil {
				return err
			}

			tx.ClearSymbols(txCtx, filePath)
			tx.ClearCalls(txCtx, filePath)
			for _, ptr := range idxResult {
				tx.SaveSymbol(txCtx, &store.Symbol{
					Name: ptr.Name, Type: ptr.Type, Doc: ptr.Doc, Path: filePath,
					StartByte: ptr.Range.Start, EndByte: ptr.Range.End,
					StartLine: ptr.StartLine, EndLine: ptr.EndLine,
				})
			}
			for _, c := range calls {
				tx.SaveCall(txCtx, store.Call{
					CallerName: c.CallerName,
					CalleeName: c.CalleeName,
					CalleePath: c.CalleePath, // Enriched Impact metadata
					LinkType:   c.LinkType,   // Enriched Impact metadata
					Path:       filePath,
					Line:       c.Line,
				})
			}
			return nil
		})

		if err != nil {
			return mcpError(fmt.Sprintf("Database update failed: %v", err)), nil
		}

		responseSymbols := idxResult
		truncated := false
		if len(idxResult) > 500 {
			responseSymbols = idxResult[:500]
			truncated = true
		}
		for i := range responseSymbols {
			responseSymbols[i].Doc = truncateDoc(responseSymbols[i].Doc)
		}
		res := map[string]interface{}{"symbols": responseSymbols, "count": len(idxResult), "truncated": truncated}
		resJSON, _ := json.Marshal(res)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_search
	searchTool := mcp.NewTool("scouter_search",
		mcp.WithDescription("Search for symbols using BM25 semantic ranking across names and docstrings."),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query (e.g. 'error handling').")),
		mcp.WithString("type", mcp.Description("Optional: symbol type filter.")),
	)

	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req SearchRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		var results []store.Symbol
		for sym, err := range db.SearchSymbolsWeighted(ctx, req.Query, req.Type) {
			if err != nil {
				return mcpError(fmt.Sprintf("Search failed: %v", err)), nil
			}
			sym.Doc = truncateDoc(sym.Doc)
			results = append(results, sym)
		}

		resJSON, _ := json.Marshal(results)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_callers
	callersTool := mcp.NewTool("scouter_callers",
		mcp.WithDescription("Find all locations where a specific symbol is called."),
		mcp.WithString("calleeName", mcp.Required(), mcp.Description("The name of the symbol being called.")),
	)

	s.AddTool(callersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req CallersRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		callers, err := db.GetCallers(ctx, req.CalleeName)
		if err != nil {
			return mcpError(fmt.Sprintf("Failed to fetch callers: %v", err)), nil
		}

		resJSON, _ := json.Marshal(callers)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_impact
	impactTool := mcp.NewTool("scouter_impact",
		mcp.WithDescription("Calculate the blast radius of changing a specific symbol."),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("The name of the symbol being changed.")),
		mcp.WithString("filePath", mcp.Description("Optional: absolute path to the file where the symbol is defined.")),
		mcp.WithNumber("maxDepth", mcp.Description("Maximum recursion depth (default 3, max 10).")),
	)

	s.AddTool(impactTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req ImpactRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		// Disambiguation Logic
		if req.FilePath == "" {
			results, err := db.SearchSymbols(ctx, req.SymbolName, "")
			if err != nil {
				return mcpError(fmt.Sprintf("Failed to disambiguate symbol: %v", err)), nil
			}

			// Filter exact matches only
			var exactMatches []string
			for _, s := range results {
				if s.Name == req.SymbolName {
					exactMatches = append(exactMatches, s.Path)
				}
			}

			if len(exactMatches) > 1 {
				return mcpError(fmt.Sprintf("Ambiguous symbol name. Found in multiple files: %s. Please provide 'filePath'.", strings.Join(exactMatches, ", "))), nil
			} else if len(exactMatches) == 1 {
				req.FilePath = exactMatches[0]
			}
		}

		impact, err := db.GetImpact(ctx, req.SymbolName, req.FilePath, req.MaxDepth)
		if err != nil {
			return mcpError(fmt.Sprintf("Impact analysis failed: %v", err)), nil
		}

		resJSON, _ := json.Marshal(impact)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_predict
	predictTool := mcp.NewTool("scouter_predict",
		mcp.WithDescription("Identify which tests to run based on current local changes (git diff)."),
		mcp.WithNumber("maxDepth", mcp.Description("Maximum impact recursion depth (default 5, max 10).")),
	)

	s.AddTool(predictTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req PredictRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		if req.MaxDepth == 0 {
			req.MaxDepth = 5
		}

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		changes, err := utils.GetLocalChanges(ctx)
		if err != nil {
			return mcpError(fmt.Sprintf("Failed to get git changes: %v", err)), nil
		}

		if len(changes) == 0 {
			return mcpJSONResponse([]byte(`{"message": "No local changes detected."}`)), nil
		}

		type Prediction struct {
			Symbol string `json:"symbol"`
			File   string `json:"file"`
			Impact []types.ImpactResult `json:"impact"`
		}
		var predictions []Prediction
		suggestedTests := make(map[string]bool)

		cwd, _ := os.Getwd()

		for _, change := range changes {
			// Convert relative git path to absolute for DB lookup
			absPath := filepath.Join(cwd, change.Path)
			symbols, err := db.GetSymbolsByRange(ctx, absPath, change.StartLine, change.EndLine)
			if err != nil {
				continue
			}

			for _, sym := range symbols {
				impact, _ := db.GetImpact(ctx, sym.Name, sym.Path, req.MaxDepth)
				
				// Identify tests in impact
				for _, imp := range impact {
					if strings.HasPrefix(imp.Symbol, "Test") || strings.HasSuffix(imp.File, "_test.go") {
						suggestedTests[imp.Symbol] = true
					}
				}

				predictions = append(predictions, Prediction{
					Symbol: sym.Name,
					File:   change.Path,
					Impact: impact,
				})
			}
		}

		testsList := make([]string, 0, len(suggestedTests))
		for t := range suggestedTests {
			testsList = append(testsList, t)
		}

		res := map[string]interface{}{
			"suggested_tests": testsList,
			"affected_symbols": predictions,
		}
		resJSON, _ := json.Marshal(res)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_critical_code
	criticalTool := mcp.NewTool("scouter_critical_code",
		mcp.WithDescription("Identify the most central and fragile symbols in the project."),
		mcp.WithNumber("limit", mcp.Description("Number of symbols to return (default 20, max 100).")),
	)

	s.AddTool(criticalTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req CriticalRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		critical, err := db.GetCriticalSymbols(ctx, req.Limit)
		if err != nil {
			return mcpError(fmt.Sprintf("Risk analysis failed: %v", err)), nil
		}

		resJSON, _ := json.Marshal(critical)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_visualize
	visualizeTool := mcp.NewTool("scouter_visualize",
		mcp.WithDescription("Generate a risk-colored Mermaid.js call graph for a specific symbol."),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("The name of the symbol to visualize.")),
		mcp.WithNumber("depth", mcp.Description("Depth of traversal (1 to 3). Default is 1.")),
		mcp.WithBoolean("riskOnly", mcp.Description("Focus Mode: Hide 'safe' nodes.")),
	)

	s.AddTool(visualizeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req VisualizeRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		if req.Depth == 0 {
			req.Depth = 1
		}
		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		// BFS Traversal
		type QueueItem struct {
			Symbol string
			Depth  int
		}
		queue := []QueueItem{{Symbol: req.SymbolName, Depth: 0}}
		visited := make(map[string]bool)
		visited[req.SymbolName] = true
		edges := make([]string, 0)
		nodeMap := make(map[string]string)
		nodeRisk := make(map[string]string)
		nodeCounter := 1

		getNodeID := func(sym string) string {
			if id, ok := nodeMap[sym]; ok {
				return id
			}

			// TASK: Calculate node risk for colorization and filtering
			risk := "safe"
			impact, _ := db.GetCallers(ctx, sym)
			centrality := len(impact)
			
			fragility := 0
			for res, err := range db.GetHealthReport(ctx, sym, true) {
				if err == nil && res.Status == "fail" { 
					fragility++ 
				}
			}

			if centrality > 10 || fragility > 0 {
				risk = "critical"
			} else if centrality > 5 {
				risk = "warn"
			}

			// DIVINE FOCUS: Filter nodes if RiskOnly is enabled
			if req.RiskOnly && risk == "safe" && sym != req.SymbolName {
				return "" // Signal to skip this node
			}

			id := fmt.Sprintf("node%d", nodeCounter)
			nodeCounter++
			nodeMap[sym] = id
			nodeRisk[id] = risk

			return id
		}
		
		// Initial root node check
		rootID := getNodeID(req.SymbolName)
		if rootID == "" {
			// If root is safe and RiskOnly is true, we still show it to provide context
			nodeMap[req.SymbolName] = "node1"
			nodeRisk["node1"] = "safe"
			nodeCounter++
		}

		for len(queue) > 0 && len(edges) < 200 {
			curr := queue[0]
			queue = queue[1:]
			if curr.Depth >= req.Depth {
				continue
			}

			callers, _ := db.GetCallers(ctx, curr.Symbol)
			for _, caller := range callers {
				if len(edges) >= 200 {
					break
				}
				
				callerID := getNodeID(caller.CallerName)
				if callerID == "" { continue } // Skip safe node in Focus Mode

				calleeID := getNodeID(curr.Symbol)
				if calleeID == "" { continue }
				
				arrow := "-->"
				if caller.LinkType == "implements" {
					arrow = "-. implements .->"
				}
				
				edge := fmt.Sprintf("    %s %s %s", callerID, arrow, calleeID)
				
				duplicate := false
				for _, e := range edges { if e == edge { duplicate = true; break } }
				
				if !duplicate {
					edges = append(edges, edge)
					if !visited[caller.CallerName] {
						visited[caller.CallerName] = true
						queue = append(queue, QueueItem{Symbol: caller.CallerName, Depth: curr.Depth + 1})
					}
				}
			}

			callees, _ := db.GetCallees(ctx, curr.Symbol)
			for _, callee := range callees {
				if len(edges) >= 200 {
					break
				}

				callerID := getNodeID(curr.Symbol)
				if callerID == "" { continue }

				calleeID := getNodeID(callee.CalleeName)
				if calleeID == "" { continue } // Skip safe node in Focus Mode
				
				arrow := "-->"
				if callee.LinkType == "implements" {
					arrow = "-. implements .->"
				}
				
				edge := fmt.Sprintf("    %s %s %s", callerID, arrow, calleeID)
				
				duplicate := false
				for _, e := range edges { if e == edge { duplicate = true; break } }

				if !duplicate {
					edges = append(edges, edge)
					if !visited[callee.CalleeName] {
						visited[callee.CalleeName] = true
						queue = append(queue, QueueItem{Symbol: callee.CalleeName, Depth: curr.Depth + 1})
					}
				}
			}
		}

		mermaidNodeDefs := "graph TD\n"
		for sym, id := range nodeMap {
			escapedSym := strings.ReplaceAll(sym, "\\", "\\\\")
			escapedSym = strings.ReplaceAll(escapedSym, "\"", "\\\"")
			mermaidNodeDefs += fmt.Sprintf("    %s[\"%s\"]\n", id, escapedSym)
		}

		styles := "\n    classDef critical fill:#f96,stroke:#333,stroke-width:4px;\n"
		styles += "    classDef warn fill:#ff9,stroke:#333,stroke-width:2px;\n"
		styles += "    classDef safe fill:#fff,stroke:#333,stroke-width:1px;\n"
		
		for id, risk := range nodeRisk {
			styles += fmt.Sprintf("    class %s %s;\n", id, risk)
		}

		resJSON, _ := json.Marshal(map[string]string{"mermaid": mermaidNodeDefs + strings.Join(edges, "\n") + styles})
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_dependencies
	depsTool := mcp.NewTool("scouter_dependencies", mcp.WithDescription("List project dependencies and versions."))
	s.AddTool(depsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deps, err := db.GetDependencies(ctx)
		if err != nil {
			return mcpError(err.Error()), nil
		}
		resJSON, _ := json.Marshal(deps)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_dead_code
	deadCodeTool := mcp.NewTool("scouter_dead_code", mcp.WithDescription("Audit the project for unused symbols."))
	s.AddTool(deadCodeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req DeadCodeRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		unused, err := db.GetUnusedSymbols(ctx, req.IncludeExported)
		if err != nil {
			return mcpError(err.Error()), nil
		}
		resJSON, _ := json.Marshal(unused)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_health
	healthTool := mcp.NewTool("scouter_health", mcp.WithDescription("List failed tests and their associated symbols."))
	s.AddTool(healthTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var results []types.TestResult
		for res, err := range db.GetHealthReport(ctx, "", true) {
			if err != nil {
				return mcpError(err.Error()), nil
			}
			results = append(results, res)
			if len(results) >= 50 {
				break
			}
		}
		resJSON, _ := json.Marshal(results)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_goto_definition
	gotoTool := mcp.NewTool("scouter_goto_definition",
		mcp.WithDescription("Jump to the exact source location of a symbol using real-time LSP intelligence."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number")),
		mcp.WithNumber("character", mcp.Required(), mcp.Description("1-based character position")),
	)
	s.AddTool(gotoTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req GotoDefinitionRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		client, err := lspMgr.GetClient(ctx, req.FilePath)
		if err != nil {
			return mcpError(fmt.Sprintf("LSP unavailable: %v", err)), nil
		}

		params := lsp.DefinitionParams{}
		params.TextDocument.URI = "file://" + req.FilePath
		params.Position.Line = req.Line - 1 // 0-based
		params.Position.Character = req.Character - 1

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		loc, err := client.Definition(timeoutCtx, params)
		if err != nil {
			return mcpError(fmt.Sprintf("LSP error: %v", err)), nil
		}

		resJSON, _ := json.Marshal(loc)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_type_info
	typeTool := mcp.NewTool("scouter_type_info",
		mcp.WithDescription("Get precise real-time type information for any variable or symbol using LSP."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number")),
		mcp.WithNumber("character", mcp.Required(), mcp.Description("1-based character position")),
	)
	s.AddTool(typeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req TypeInfoRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		client, err := lspMgr.GetClient(ctx, req.FilePath)
		if err != nil {
			return mcpError(fmt.Sprintf("LSP unavailable: %v", err)), nil
		}

		params := lsp.HoverParams{}
		params.TextDocument.URI = "file://" + req.FilePath
		params.Position.Line = req.Line - 1
		params.Position.Character = req.Character - 1

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		hover, err := client.Hover(timeoutCtx, params)
		if err != nil {
			return mcpError(fmt.Sprintf("LSP error: %v", err)), nil
		}

		resJSON, _ := json.Marshal(hover)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_read
	readTool := mcp.NewTool("scouter_read", mcp.WithDescription("Read a specific code fragment with integrity verification."))
	s.AddTool(readTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req ReadRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)
		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}
		pointerJSON, _ := json.Marshal(req.Pointer)
		fragment, err := engine.ReadFragment(ctx, req.FilePath, string(pointerJSON), req.Hash)
		if err != nil {
			return mcpError(err.Error()), nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fragment}}}, nil
	})

	// Prompt: scouter-explain
	explainPrompt := mcp.NewPrompt("scouter-explain",
		mcp.WithPromptDescription("Find and explain a symbol in the project."),
		mcp.WithArgument("symbolName", mcp.ArgumentDescription("Symbol to explain"), mcp.RequiredArgument()),
	)
	s.AddPrompt(explainPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		symbolName := request.Params.Arguments["symbolName"]
		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Explain symbol %s", symbolName),
			Messages: []mcp.PromptMessage{{
				Role:    mcp.RoleUser,
				Content: mcp.NewTextContent(fmt.Sprintf(`I need to understand '%s'. Use 'scouter_search', then 'scouter_callers' to see its usage, 'scouter_read' the code, and explain it.`, symbolName)),
			}},
		}, nil
	})

	// Launch server in a goroutine to allow signal handling
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting MCP server v%s (stdio)...", version)
		if err := server.ServeStdio(s); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received. Cleaning up...")
	case err := <-errChan:
		log.Fatalf("MCP server error: %v", err)
	}
}

func mcpError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: msg}}, IsError: true}
}

func mcpJSONResponse(data []byte) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(data)}}}
}

func truncateDoc(doc string) string {
	if len(doc) > 1000 {
		return doc[:997] + "..."
	}
	return doc
}

func installGeminiCLI() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".gemini", "settings.json")
	binPath, _ := filepath.Abs(os.Args[0])

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp"},
		"trust":   true,
	}

	newData, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		fmt.Printf("❌ Error saving config: %v\n", err)
		return
	}
	fmt.Printf("✅ Scouter integrated with Gemini CLI (with trust: true)!\n")
}

func installOpenCode() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "settings.json")
	binPath, _ := os.Executable()
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}
	mcpServers["scouter"] = map[string]interface{}{"type": "local", "command": []string{binPath, "mcp"}, "enabled": true}
	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0644)
	fmt.Printf("✅ Scouter integrated with OpenCode!\n")
}
