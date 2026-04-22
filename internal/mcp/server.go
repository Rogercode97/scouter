package mcp

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/Rogercode97/scouter/internal/store"
)

// MCPMessage represents a JSON-RPC message.
type MCPMessage struct {
	ID     any            `json:"id,omitempty"`
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// Server implements the Model Context Protocol (MCP).
type Server struct {
	store    store.Repository
	resolver *PointerResolver
	wg       sync.WaitGroup
	mu       sync.Mutex
	enc      *json.Encoder
	tools    []map[string]any
}

func NewServer(st store.Repository) *Server {
	s := &Server{
		store:    st,
		resolver: NewPointerResolver(st),
		enc:      json.NewEncoder(os.Stdout),
	}
	s.initTools()
	return s
}

func (s *Server) initTools() {
	s.tools = []map[string]any{
		{
			"name":        "scouter_index",
			"description": "Index a file or directory for AST symbols",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
				},
				"required": []string{"filePath"},
			},
		},
		{
			"name":        "scouter_search",
			"description": "Search for symbols using semantic or text search",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"type":  map[string]any{"type": "string", "enum": []string{"function", "method", "class", "interface"}},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "scouter_read",
			"description": "Read a specific symbol or fragment from a file",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
					"pointer":  map[string]any{"type": "string", "description": "symbol name, range json, or hash"},
				},
				"required": []string{"filePath", "pointer"},
			},
		},
		{
			"name":        "scouter_callers",
			"description": "Find all callers of a given function or method",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"calleeName": map[string]any{"type": "string"},
				},
				"required": []string{"calleeName"},
			},
		},
		{
			"name":        "scouter_impact",
			"description": "Calculate the impact (blast radius) of changing a symbol",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbolName": map[string]any{"type": "string"},
					"filePath":   map[string]any{"type": "string"},
					"maxDepth":   map[string]any{"type": "integer"},
				},
				"required": []string{"symbolName", "filePath"},
			},
		},
		{
			"name":        "scouter_critical_code",
			"description": "Identify high-risk symbols (high centrality and fragility)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer"},
				},
			},
		},
		{
			"name":        "scouter_dependencies",
			"description": "Get a map of all project dependencies",
			"inputSchema": map[string]any{
				"type": "object",
			},
		},
		{
			"name":        "scouter_pure_signal",
			"description": "Extract Pure Signal from text using RTK synergy",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text":  map[string]any{"type": "string"},
					"mode":  map[string]any{"type": "string", "enum": []string{"log", "read"}},
					"level": map[string]any{"type": "string", "enum": []string{"light", "moderate", "aggressive"}},
				},
				"required": []string{"text"},
			},
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	decoder := json.NewDecoder(os.Stdin)
	msgChan := make(chan MCPMessage)
	errChan := make(chan error, 1)

	go func() {
		for {
			var msg MCPMessage
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

	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return ctx.Err()
		case <-errChan:
			s.wg.Wait()
			return nil
		case msg := <-msgChan:
			switch msg.Method {
			case "initialize":
				s.sendResponse(msg.ID, map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
					"serverInfo": map[string]any{"name": "scouter", "version": "2.6.2-final"},
				})
			case "tools/list":
				s.sendResponse(msg.ID, map[string]any{"tools": s.tools})
			case "tools/call":
				toolName, _ := msg.Params["name"].(string)
				args, ok := msg.Params["arguments"].(map[string]any)
				if !ok {
					args = make(map[string]any)
				}

				s.wg.Add(1)
				go func(id any, toolName string, args map[string]any) {
					defer s.wg.Done()
					s.dispatchToolCall(ctx, id, toolName, args)
				}(msg.ID, toolName, args)
			}
		}
	}
}

func (s *Server) sendResponse(id any, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	_ = s.enc.Encode(resp)
}

func (s *Server) sendError(id any, code int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	_ = s.enc.Encode(resp)
}

func (s *Server) dispatchToolCall(ctx context.Context, id any, name string, args map[string]any) {
	var result any
	var err error
	switch name {
	case "scouter_index": result, err = s.handleIndex(ctx, args)
	case "scouter_search": result, err = s.handleSearch(ctx, args)
	case "scouter_read": result, err = s.handleRead(ctx, args)
	case "scouter_callers": result, err = s.handleCallers(ctx, args)
	case "scouter_impact": result, err = s.handleImpact(ctx, args)
	case "scouter_critical_code": result, err = s.handleCritical(ctx, args)
	case "scouter_dependencies": result, err = s.handleDependencies(ctx, args)
	case "scouter_pure_signal": result, err = s.handlePureSignal(ctx, args)
	default:
		s.sendError(id, -32601, "method not found: "+name)
		return
	}
	if err != nil {
		s.sendError(id, -32603, err.Error())
		return
	}
	s.sendResponse(id, result)
}
