package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/store"
)

// Server implements the Model Context Protocol (MCP).
type Server struct {
	store    store.Repository
	resolver *PointerResolver
	encoder  *json.Encoder
}

func NewServer(st store.Repository) *Server {
	return &Server{
		store:    st,
		resolver: NewPointerResolver(st),
		encoder:  json.NewEncoder(os.Stdout),
	}
}

func (s *Server) Run(ctx context.Context) error {
	decoder := json.NewDecoder(os.Stdin)
	msgChan := make(chan map[string]interface{})
	errChan := make(chan error, 1)

	// Background reader for blocking I/O
	go func() {
		for {
			var msg map[string]interface{}
			if err := decoder.Decode(&msg); err != nil {
				errChan <- err
				return
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Event loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-errChan:
			return nil
		case msg := <-msgChan:
			method, _ := msg["method"].(string)
			params, _ := msg["params"].(map[string]interface{})
			id, _ := msg["id"]

			switch method {
			case "initialize":
				s.sendResponse(id, map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"serverInfo": map[string]string{"name": "scouter", "version": "2.6.0"},
				})
			case "tools/list":
				s.sendResponse(id, map[string]interface{}{"tools": s.getToolsList()})
			case "tools/call":
				toolName, _ := params["name"].(string)
				args, _ := params["arguments"].(map[string]interface{})
				s.dispatchToolCall(ctx, id, toolName, args)
			}
		}
	}
}

func (s *Server) dispatchToolCall(ctx context.Context, id interface{}, name string, args map[string]interface{}) {
	var result interface{}
	var err error

	switch name {
	case "scouter_index":
		result, err = s.handleIndex(ctx, args)
	case "scouter_search":
		result, err = s.handleSearch(ctx, args)
	case "scouter_read":
		result, err = s.handleRead(ctx, args)
	case "scouter_pure_signal":
		result, err = s.handlePureSignal(ctx, args)
	case "scouter_callers":
		result, err = s.handleCallers(ctx, args)
	case "scouter_impact":
		result, err = s.handleImpact(ctx, args)
	case "scouter_critical_code":
		result, err = s.handleCritical(ctx, args)
	case "scouter_dependencies":
		result, err = s.handleDependencies(ctx, args)
	default:
		s.sendError(id, fmt.Sprintf("tool not found: %s", name))
		return
	}

	if err != nil {
		s.sendError(id, err.Error())
	} else {
		s.sendResponse(id, result)
	}
}

func (s *Server) getToolsList() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "scouter_index",
			"description": "Index a file for structural analysis",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{"type": "string"},
				},
				"required": []string{"filePath"},
			},
		},
		{
			"name":        "scouter_search",
			"description": "Search for symbols (functions, classes, interfaces)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
					"type":  map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "scouter_read",
			"description": "Read a code fragment surgically using a pointer",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{"type": "string"},
					"pointer":  map[string]interface{}{"type": "string"},
				},
				"required": []string{"filePath", "pointer"},
			},
		},
		{
			"name":        "scouter_pure_signal",
			"description": "Purify noisy text (logs or code) using the Rust Token Killer (rtk) engine.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{"type": "string", "description": "The raw noisy text to purify"},
					"mode": map[string]interface{}{"type": "string", "description": "Processing mode: 'log' or 'read'. Default: auto-detect."},
					"level": map[string]interface{}{"type": "string", "description": "Filtering level: 'minimal' or 'aggressive'. Default: aggressive."},
				},
				"required": []string{"text"},
			},
		},
		{
			"name":        "scouter_callers",
			"description": "Find callers of a specific symbol",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"calleeName": map[string]interface{}{"type": "string"},
				},
				"required": []string{"calleeName"},
			},
		},
		{
			"name":        "scouter_impact",
			"description": "Analyze blast radius of a change",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{"type": "string"},
					"filePath":   map[string]interface{}{"type": "string"},
					"maxDepth":   map[string]interface{}{"type": "integer"},
				},
				"required": []string{"symbolName"},
			},
		},
		{
			"name":        "scouter_critical_code",
			"description": "Identify code with highest centrality and impact",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "integer"},
				},
			},
		},
		{
			"name":        "scouter_dependencies",
			"description": "List all tracked inter-file dependencies",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
				},
			},
		},
	}
}

func (s *Server) sendResponse(id interface{}, result interface{}) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	_ = s.encoder.Encode(resp)
}

func (s *Server) sendError(id interface{}, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    -32603,
			"message": message,
		},
	}
	_ = s.encoder.Encode(resp)
}
