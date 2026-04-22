package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
)

// ToolHandler defines the function signature for an MCP tool.
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

func (s *Server) handleIndex(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	filePath, _ := args["filePath"].(string)
	if filePath == "" {
		return nil, fmt.Errorf("missing filePath")
	}

	path, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	_, _, err = engine.ParseFile(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "✅ Indexed " + filePath},
		},
	}, nil
}

func (s *Server) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	symbolType, _ := args["type"].(string)

	results, err := s.store.SearchSymbols(ctx, query, symbolType)
	if err != nil {
		return nil, err
	}

	out, _ := json.Marshal(results)
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(out)},
		},
	}, nil
}

func (s *Server) handleRead(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	filePath, _ := args["filePath"].(string)
	pointer, _ := args["pointer"].(string)

	if filePath == "" || pointer == "" {
		return nil, fmt.Errorf("missing filePath or pointer")
	}

	path, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	// Use the resolver to get the range
	rng, err := s.resolver.Resolve(ctx, path, pointer)
	if err != nil {
		return nil, err
	}

	content, err := engine.ReadFragment(ctx, path, rng)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": content},
		},
	}, nil
}

func (s *Server) handleCallers(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	calleeName, _ := args["calleeName"].(string)
	if calleeName == "" {
		return nil, fmt.Errorf("missing calleeName")
	}
	results, err := s.store.GetCallers(ctx, calleeName)
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(results)
	return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": string(out)}}}, nil
}

func (s *Server) handleImpact(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbolName, _ := args["symbolName"].(string)
	filePath, _ := args["filePath"].(string)
	maxDepth := 5
	if d, ok := args["maxDepth"].(float64); ok {
		maxDepth = int(d)
	}
	results, err := s.store.GetImpact(ctx, symbolName, filePath, maxDepth)
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(results)
	return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": string(out)}}}, nil
}

func (s *Server) handleCritical(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	results, err := s.store.GetCriticalSymbols(ctx, limit)
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(results)
	return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": string(out)}}}, nil
}

func (s *Server) handleDependencies(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	res, err := s.store.GetDependencies(ctx)
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(res)
	return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": string(out)}}}, nil
}

func (s *Server) handlePureSignal(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("missing 'text' argument")
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		// Heuristic: if it contains function keywords or braces, it's code. Otherwise log.
		if strings.Contains(text, "func ") || strings.Contains(text, "package ") || strings.HasPrefix(strings.TrimSpace(text), "import ") {
			mode = "read"
		} else {
			mode = "log"
		}
	}

	var cmd *exec.Cmd
	switch mode {
	case "log":
		cmd = exec.CommandContext(ctx, "rtk", "log")
	case "read":
		level, _ := args["level"].(string)
		if level == "" {
			level = "aggressive"
		}
		cmd = exec.CommandContext(ctx, "rtk", "read", "-", "--level", level)
	default:
		return nil, fmt.Errorf("invalid mode: %s (supported: log, read)", mode)
	}

	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("rtk not found, please install it via 'brew install rtk'")
		}
		// Return output even on error as RTK might have filtered partially
		if len(out) == 0 {
			return nil, fmt.Errorf("rtk execution failed (%s): %w", mode, err)
		}
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(out)},
		},
	}, nil
}
