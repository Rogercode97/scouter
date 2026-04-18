package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/go-playground/validator/v10"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var version = "1.2.0" // Bumping version for V2.0 features

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

type VisualizeRequest struct {
	SymbolName string `json:"symbolName" validate:"required,min=1"`
	Depth      int    `json:"depth" validate:"omitempty,min=1,max=3"`
}

func runMCPServer() {
	ctx := context.Background()
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
	defer db.Close()

	v := validator.New(validator.WithRequiredStructEnabled())

	s := server.NewMCPServer("scouter", version,
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithInstructions(`Scouter is an AST-based code analysis engine. 
Use 'scouter_index' to understand a file's structure and 'scouter_search' for high-precision symbol lookups.
Use 'scouter_callers' to find all locations where a specific function or method is invoked across the workspace.
Use 'scouter_visualize' to generate a Mermaid.js call graph for a symbol up to a specified depth.
Prefer 'scouter_search' and 'scouter_callers' over generic text search (grep) for architectural analysis.
Use 'scouter_read' with pointers to read specific code fragments with integrity verification (hash).`),
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
		mcp.WithDescription("Analyze a file to index its structure (symbols) and call graph (invocations)."),
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
				res := map[string]interface{}{"symbols": responseSymbols, "count": len(cachedSymbols), "truncated": truncated, "cached": true}
				resJSON, _ := json.Marshal(res)
				return mcpJSONResponse(resJSON), nil
			}
		}

		idxResult, calls, err := engine.ParseFile(ctx, filePath)
		if err != nil {
			return mcpError(fmt.Sprintf("Indexing failed: %v", err)), nil
		}

		err = db.WithTransaction(ctx, func(tx store.Repository) error {
			astJSON, _ := json.Marshal(idxResult)
			if err := tx.SaveFileIndex(ctx, &store.FileIndex{
				Path: filePath, Mtime: stats.ModTime().UnixNano(), Hash: currentHash, ASTJSON: string(astJSON),
			}); err != nil { return err }

			tx.ClearSymbols(ctx, filePath)
			tx.ClearCalls(ctx, filePath)
			for _, ptr := range idxResult {
				tx.SaveSymbol(ctx, &store.Symbol{
					Name: ptr.Name, Type: ptr.Type, Path: filePath,
					StartByte: ptr.Range.Start, EndByte: ptr.Range.End,
					StartLine: ptr.StartLine, EndLine: ptr.EndLine,
				})
			}
			for _, c := range calls {
				tx.SaveCall(ctx, store.Call{CallerName: c.CallerName, CalleeName: c.CalleeName, Path: filePath, Line: c.Line})
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
		res := map[string]interface{}{"symbols": responseSymbols, "count": len(idxResult), "truncated": truncated}
		resJSON, _ := json.Marshal(res)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_search
	searchTool := mcp.NewTool("scouter_search",
		mcp.WithDescription("Search for symbols across the indexed workspace using FTS5."),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query (e.g. 'ValidateUser').")),
		mcp.WithString("type", mcp.Description("Optional: symbol type filter.")),
	)

	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req SearchRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		json.Unmarshal(argsJSON, &req)

		if err := v.Struct(req); err != nil {
			return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
		}

		results, err := db.SearchSymbols(ctx, req.Query, req.Type)
		if err != nil {
			return mcpError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		resJSON, _ := json.Marshal(results)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_callers
	callersTool := mcp.NewTool("scouter_callers",
		mcp.WithDescription("Find all locations where a specific symbol (function/method) is called."),
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

	// Tool: scouter_visualize
	visualizeTool := mcp.NewTool("scouter_visualize",
		mcp.WithDescription("Generate a Mermaid.js call graph for a specific symbol up to a specified depth."),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("The name of the symbol to visualize.")),
		mcp.WithNumber("depth", mcp.Description("Depth of traversal (1 to 3). Default is 1.")),
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
		
		edges := make(map[string]bool) // Unique edges: "caller->callee"
		nodeMap := make(map[string]string) // symbolName -> nodeID
		nodeCounter := 1

		getNodeID := func(sym string) string {
			if id, ok := nodeMap[sym]; ok {
				return id
			}
			id := fmt.Sprintf("node%d", nodeCounter)
			nodeCounter++
			nodeMap[sym] = id
			return id
		}

		// Limit total nodes/edges to prevent massive graphs (OOM Guard)
		maxElements := 200

		// Ensure the root node is always in the map
		getNodeID(req.SymbolName)

		for len(queue) > 0 && len(edges) < maxElements {
			curr := queue[0]
			queue = queue[1:]

			if curr.Depth >= req.Depth {
				continue
			}

			// Get incoming calls (Callers)
			callers, _ := db.GetCallers(ctx, curr.Symbol)
			for _, caller := range callers {
				if len(edges) >= maxElements {
					break
				}
				edgeKey := fmt.Sprintf("%s->%s", caller.CallerName, curr.Symbol)
				if !edges[edgeKey] {
					edges[edgeKey] = true
					if !visited[caller.CallerName] {
						visited[caller.CallerName] = true
						queue = append(queue, QueueItem{Symbol: caller.CallerName, Depth: curr.Depth + 1})
					}
				}
			}

			// Get outgoing calls (Callees)
			callees, _ := db.GetCallees(ctx, curr.Symbol)
			for _, callee := range callees {
				if len(edges) >= maxElements {
					break
				}
				edgeKey := fmt.Sprintf("%s->%s", curr.Symbol, callee.CalleeName)
				if !edges[edgeKey] {
					edges[edgeKey] = true
					if !visited[callee.CalleeName] {
						visited[callee.CalleeName] = true
						queue = append(queue, QueueItem{Symbol: callee.CalleeName, Depth: curr.Depth + 1})
					}
				}
			}
		}

		// Generate Mermaid
		mermaidEdges := ""
		for edge := range edges {
			// Find separator index
			var caller, callee string
			for i := 0; i < len(edge)-2; i++ {
				if edge[i:i+2] == "->" {
					caller = edge[:i]
					callee = edge[i+2:]
					break
				}
			}
			callerID := getNodeID(caller)
			calleeID := getNodeID(callee)
			mermaidEdges += fmt.Sprintf("    %s --> %s\n", callerID, calleeID)
		}
		
		mermaidNodeDefs := "graph TD\n"
		for sym, id := range nodeMap {
			// Escape quotes and backslashes in symbol names
			escapedSym := strings.ReplaceAll(sym, "\\", "\\\\")
			escapedSym = strings.ReplaceAll(escapedSym, "\"", "\\\"")
			mermaidNodeDefs += fmt.Sprintf("    %s[\"%s\"]\n", id, escapedSym)
		}

		finalMermaid := mermaidNodeDefs + mermaidEdges

		res := map[string]string{
			"mermaid": finalMermaid,
		}
		resJSON, _ := json.Marshal(res)
		return mcpJSONResponse(resJSON), nil
	})

	// Tool: scouter_read
	readTool := mcp.NewTool("scouter_read",
		mcp.WithDescription("Read a specific code fragment with integrity verification."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the file.")),
		mcp.WithObject("pointer", mcp.Required(), mcp.Description("AST pointer range.")),
		mcp.WithString("hash", mcp.Description("Expected SHA-256 hash of the fragment.")),
	)

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
			return mcpError(fmt.Sprintf("Read failed: %v", err)), nil
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
				Role: mcp.RoleUser,
				Content: mcp.NewTextContent(fmt.Sprintf(`I need to understand '%s'. Use 'scouter_search', then 'scouter_callers' to see its usage, 'scouter_read' the code, and explain it.`, symbolName)),
			}},
		}, nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server failed: %v", err)
	}
}

func mcpError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: msg}}, IsError: true}
}

func mcpJSONResponse(data []byte) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(data)}}}
}

func installGeminiCLI() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".gemini", "settings.json")
	binPath, _ := os.Executable()
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil { json.Unmarshal(data, &config) } else { config = make(map[string]interface{}) }
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok { mcpServers = make(map[string]interface{}); config["mcpServers"] = mcpServers }
	mcpServers["scouter"] = map[string]interface{}{"command": []string{binPath, "mcp"}, "enabled": true}
	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0644)
	fmt.Printf("✅ Scouter integrated with Gemini CLI!\n")
}

func installOpenCode() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "settings.json")
	binPath, _ := os.Executable()
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil { json.Unmarshal(data, &config) } else { config = make(map[string]interface{}) }
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok { mcpServers = make(map[string]interface{}); config["mcpServers"] = mcpServers }
	mcpServers["scouter"] = map[string]interface{}{"type": "local", "command": []string{binPath, "mcp"}, "enabled": true}
	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0644)
	fmt.Printf("✅ Scouter integrated with OpenCode!\n")
}
