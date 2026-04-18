package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/go-playground/validator/v10"
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
Use 'scouter_index' to understand a file's structure and 'scouter_search' for high-precision symbol lookups (functions, classes, etc.). 
Prefer 'scouter_search' over generic text search (grep) when looking for definitions. 
Use 'scouter_read' with pointers obtained from index or search for surgical, byte-safe code reading.
Always index a file before attempting to read specific fragments if you don't have a valid pointer.`),
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
		mcp.WithDescription("Analyze a file using the AST engine to index its structure (classes, methods, functions, variables)."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The absolute path to the file to index."),
		),
	)

	s.AddTool(indexTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req IndexRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		if err := json.Unmarshal(argsJSON, &req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid arguments"}},
				IsError: true,
			}, nil
		}

		if err := v.Struct(req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Validation failed: %v", err)}},
				IsError: true,
			}, nil
		}

		filePath := req.FilePath
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
			var cachedSymbols []types.ASTPointer
			if err := json.Unmarshal([]byte(cached.ASTJSON), &cachedSymbols); err == nil {
				// OOM Guard for cache hits
				responseSymbols := cachedSymbols
				truncated := false
				if len(cachedSymbols) > 500 {
					responseSymbols = cachedSymbols[:500]
					truncated = true
				}

				res := map[string]interface{}{
					"symbols":   responseSymbols,
					"count":     len(cachedSymbols),
					"truncated": truncated,
					"cached":    true,
				}
				resJSON, _ := json.Marshal(res)
				return &mcp.CallToolResult{
					Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(resJSON)}},
				}, nil
			}
		}

		idxResult, err := engine.ParseFile(ctx, filePath)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Indexing failed: %v", err)}},
				IsError: true,
			}, nil
		}

		// Use Transaction for Atomicity
		err = db.WithTransaction(ctx, func(tx store.Repository) error {
			astJSON, _ := json.Marshal(idxResult)
			if err := tx.SaveFileIndex(ctx, &store.FileIndex{
				Path:    filePath,
				Mtime:   stats.ModTime().UnixNano(),
				Hash:    currentHash,
				ASTJSON: string(astJSON),
			}); err != nil {
				return err
			}

			// Save individual symbols for FTS5 search
			if err := tx.ClearSymbols(ctx, filePath); err != nil {
				return err
			}
			for _, ptr := range idxResult {
				if err := tx.SaveSymbol(ctx, &store.Symbol{
					Name:      ptr.Name,
					Type:      ptr.Type,
					Path:      filePath,
					StartByte: ptr.Range.Start,
					EndByte:   ptr.Range.End,
					StartLine: ptr.StartLine,
					EndLine:   ptr.EndLine,
				}); err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Database update failed: %v", err)}},
				IsError: true,
			}, nil
		}

		// OOM Guard: Limit symbols in response
		responseSymbols := idxResult
		truncated := false
		if len(idxResult) > 500 {
			responseSymbols = idxResult[:500]
			truncated = true
		}

		res := map[string]interface{}{
			"symbols":   responseSymbols,
			"count":     len(idxResult),
			"truncated": truncated,
		}
		resJSON, _ := json.Marshal(res)

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(resJSON)}},
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
		var req SearchRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		if err := json.Unmarshal(argsJSON, &req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid arguments"}},
				IsError: true,
			}, nil
		}

		if err := v.Struct(req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Validation failed: %v", err)}},
				IsError: true,
			}, nil
		}

		results, err := db.SearchSymbols(ctx, req.Query, req.Type)
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
		mcp.WithDescription("Read a specific code fragment from a file using an AST pointer (byte-safe start/end positions)."),
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
		var req ReadRequest
		argsJSON, _ := json.Marshal(request.GetArguments())
		if err := json.Unmarshal(argsJSON, &req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Invalid arguments"}},
				IsError: true,
			}, nil
		}

		if err := v.Struct(req); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Validation failed: %v", err)}},
				IsError: true,
			}, nil
		}
		
		pointerJSON, _ := json.Marshal(req.Pointer)
		fragment, err := engine.ReadFragment(ctx, req.FilePath, string(pointerJSON), req.Hash)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Read failed: %v", err)}},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fragment}},
		}, nil
	})

	// Prompt: scouter-explain
	explainPrompt := mcp.NewPrompt("scouter-explain",
		mcp.WithPromptDescription("Find and explain a symbol in the project."),
		mcp.WithArgument("symbolName",
			mcp.ArgumentDescription("The name of the symbol to explain (e.g. 'ValidateUser')"),
			mcp.RequiredArgument(),
		),
	)

	s.AddPrompt(explainPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		symbolName := request.Params.Arguments["symbolName"]
		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Explain symbol %s", symbolName),
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.NewTextContent(fmt.Sprintf(`I need to understand how '%s' is implemented and used. 
Please use 'scouter_search' to find it, then 'scouter_read' the relevant fragment, and finally explain its purpose and logic.`, symbolName)),
				},
			},
		}, nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server failed: %v", err)
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
