package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var version = "1.1.0"

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

func runMCPServer() {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".scouter", "scouter.db")
	db, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer db.Close()

	s := server.NewMCPServer("scouter", version,
		server.WithToolCapabilities(true),
	)

	// Tool: scouter_index
	indexTool := mcp.NewTool("scouter_index",
		mcp.WithDescription("Analyze a file using the AST engine to index its structure (classes, methods, functions, variables)."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The absolute path to the file to index."),
		),
	)

	s.AddTool(indexTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath := request.GetString("filePath", "")
		if filePath == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid filePath"}},
				IsError: true,
			}, nil
		}

		stats, err := os.Stat(filePath)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("File not found: %v", err)}},
				IsError: true,
			}, nil
		}

		currentHash, _ := utils.CalculateHash(filePath)

		cached, err := db.GetFileIndex(ctx, filePath)
		if err == nil && (cached.Mtime == stats.ModTime().UnixNano() || (currentHash != "" && cached.Hash == currentHash)) {
			log.Printf("Cache hit for %s (mtime: %v, hash: %v)", filePath, cached.Mtime == stats.ModTime().UnixNano(), currentHash != "" && cached.Hash == currentHash)
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: cached.ASTJSON}},
			}, nil
		}

		idxResult, err := engine.ParseFile(ctx, filePath)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Indexing failed: %v", err)}},
				IsError: true,
			}, nil
		}

		astJSON, _ := json.Marshal(idxResult)
		db.SaveFileIndex(ctx, &store.FileIndex{
			Path:    filePath,
			Mtime:   stats.ModTime().UnixNano(),
			Hash:    currentHash,
			ASTJSON: string(astJSON),
		})

		// Save individual symbols for FTS5 search
		db.ClearSymbols(ctx, filePath)
		for _, ptr := range idxResult {
			db.SaveSymbol(ctx, &store.Symbol{
				Name:      ptr.Name,
				Type:      ptr.Type,
				Path:      filePath,
				StartByte: ptr.Range.Start,
				EndByte:   ptr.Range.End,
				StartLine: ptr.StartLine,
				EndLine:   ptr.EndLine,
			})
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(astJSON)}},
		}, nil
	})

	// Tool: scouter_search
	searchTool := mcp.NewTool("scouter_search",
		mcp.WithDescription("Search for symbols (functions, classes, variables) across the indexed workspace using FTS5."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query (e.g. 'ValidateUser', 'Config')."),
		),
		mcp.WithString("type",
			mcp.Description("Optional: Filter by symbol type (function, class, variable, method, interface)."),
		),
	)

	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetString("query", "")
		symType := request.GetString("type", "")

		if query == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid query"}},
				IsError: true,
			}, nil
		}

		results, err := db.SearchSymbols(ctx, query, symType)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Search failed: %v", err)}},
				IsError: true,
			}, nil
		}

		resJSON, _ := json.Marshal(results)
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(resJSON)}},
		}, nil
	})

	// Tool: scouter_read
	readTool := mcp.NewTool("scouter_read",
		mcp.WithDescription("Read a specific code snippet from a file using an AST pointer (byte-safe start/end positions)."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The absolute path to the file."),
		),
		mcp.WithObject("pointer",
			mcp.Required(),
			mcp.Description("The AST pointer object containing position metadata."),
		),
	)

	s.AddTool(readTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath := request.GetString("filePath", "")
		if filePath == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid filePath"}},
				IsError: true,
			}, nil
		}
		
		args := request.GetArguments()
		pointerJSON, _ := json.Marshal(args["pointer"])
		
		snippet, err := engine.ReadSnippet(ctx, filePath, string(pointerJSON))
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Read failed: %v", err)}},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: snippet}},
		}, nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server failed: %v", err)
	}
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

func installGeminiCLI() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".gemini", "settings.json")
	binPath, _ := os.Executable()

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read Gemini config: %v", err)
	}

	var config map[string]interface{}
	json.Unmarshal(data, &config)

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp"},
	}

	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0644)
	fmt.Printf("✅ Scouter integrated with Gemini CLI!\n")
}

func installOpenCode() {
	home, _ := os.UserHomeDir()
	pluginDir := filepath.Join(home, ".config", "opencode", "plugins")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	binPath, _ := os.Executable()

	// 1. Install TypeScript plugin (extracted from binary)
	os.MkdirAll(pluginDir, 0755)
	pluginData, err := openCodePluginFS.ReadFile("plugins/opencode/scouter.ts")
	if err != nil {
		log.Fatalf("Failed to read embedded plugin: %v", err)
	}
	os.WriteFile(filepath.Join(pluginDir, "scouter.ts"), pluginData, 0644)

	// 2. Register MCP server in opencode.json
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If opencode.json doesn't exist, create an empty one
		data = []byte("{}")
	}

	var config map[string]interface{}
	json.Unmarshal(data, &config)

	mcpBlock, ok := config["mcp"].(map[string]interface{})
	if !ok {
		mcpBlock = make(map[string]interface{})
		config["mcp"] = mcpBlock
	}

	mcpBlock["scouter"] = map[string]interface{}{
		"type":    "local",
		"command": []string{binPath, "mcp"},
		"enabled": true,
	}

	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0644)
	fmt.Printf("✅ Scouter integrated with OpenCode!\n")
}
