package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// LSPClient defines the interface for an LSP client.
type LSPClient interface {
	Definition(ctx context.Context, params DefinitionParams) ([]Location, error)
	Implementation(ctx context.Context, params ImplementationParams) ([]Location, error)
	References(ctx context.Context, params ReferenceParams) ([]Location, error)
	Hover(ctx context.Context, params HoverParams) (*Hover, error)

	// Call Hierarchy (Omniscience V2)
	PrepareCallHierarchy(ctx context.Context, params CallHierarchyPrepareParams) ([]CallHierarchyItem, error)
	IncomingCalls(ctx context.Context, params CallHierarchyIncomingCallsParams) ([]CallHierarchyIncomingCall, error)
	OutgoingCalls(ctx context.Context, params CallHierarchyOutgoingCallsParams) ([]CallHierarchyOutgoingCall, error)

	Close() error
}

type jsonrpcClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader io.Reader // for testing
	writer io.Writer // for testing

	nextID  atomic.Uint64
	pending sync.Map // map[uint64]chan *JSONRPCResponse

	done chan struct{}
}

func NewClient(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &jsonrpcClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: stdout,
		writer: stdin,
		done:   make(chan struct{}),
	}

	go c.listen()

	// Initialize
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

func (c *jsonrpcClient) listen() {
	reader := bufio.NewReader(c.reader)
	for {
		// Read headers
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				close(c.done)
				return
			}
			line = strings.TrimRight(line, "\r\n") // Fix: Protocol violation (handle \r\n)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Fix: OOM Vulnerability (cap at 5MB)
		if contentLength > 5*1024*1024 {
			close(c.done)
			return // Disconnect on massive payload
		}

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			close(c.done)
			return
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if resp.ID != nil {
			var id uint64
			switch v := resp.ID.(type) {
			case float64:
				id = uint64(v)
			case string:
				id, _ = strconv.ParseUint(v, 10, 64)
			}

			if ch, ok := c.pending.LoadAndDelete(id); ok {
				ch.(chan *JSONRPCResponse) <- &resp
			}
		}
	}
}

func (c *jsonrpcClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.nextID.Add(1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}

	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return err
		}
		req.Params = p
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	ch := make(chan *JSONRPCResponse, 1)
	c.pending.Store(id, ch)

	payload := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	if _, err := fmt.Fprint(c.writer, payload); err != nil {
		c.pending.Delete(id)
		return err
	}

	select {
	case <-ctx.Done():
		c.pending.Delete(id)
		return ctx.Err()
	case <-c.done:
		return io.EOF
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("LSP error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		if result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

func (c *jsonrpcClient) initialize(ctx context.Context) error {
	cwd, _ := os.Getwd()
	params := InitializeParams{
		ProcessID:    os.Getpid(),
		RootURI:      "file://" + cwd,
		Capabilities: ClientCapabilities{},
		WorkspaceFolders: []WorkspaceFolder{
			{
				Name: "scouter",
				URI:  "file://" + cwd,
			},
		},
	}
	params.Capabilities.TextDocument.Hover.ContentFormat = []string{"markdown", "plaintext"}

	var result interface{}
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return err
	}

	// Send initialized notification
	initializedReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(initializedReq)
	payload := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	fmt.Fprint(c.writer, payload)

	return nil
}

func (c *jsonrpcClient) Definition(ctx context.Context, params DefinitionParams) ([]Location, error) {
	var locs []Location
	// The response can be Location | Location[] | LocationLink[] | null
	// We handle Location and []Location for now.
	var raw json.RawMessage
	if err := c.call(ctx, "textDocument/definition", params, &raw); err != nil {
		return nil, err
	}

	if string(raw) == "null" {
		return nil, nil
	}

	// Try single Location
	var loc Location
	if err := json.Unmarshal(raw, &loc); err == nil {
		return []Location{loc}, nil
	}

	// Try slice
	if err := json.Unmarshal(raw, &locs); err == nil {
		return locs, nil
	}

	return nil, fmt.Errorf("unexpected definition response: %s", string(raw))
}

func (c *jsonrpcClient) Implementation(ctx context.Context, params ImplementationParams) ([]Location, error) {
	var locs []Location
	var raw json.RawMessage
	if err := c.call(ctx, "textDocument/implementation", params, &raw); err != nil {
		return nil, err
	}

	if string(raw) == "null" {
		return nil, nil
	}

	// Try single Location
	var loc Location
	if err := json.Unmarshal(raw, &loc); err == nil {
		return []Location{loc}, nil
	}

	// Try slice
	if err := json.Unmarshal(raw, &locs); err == nil {
		return locs, nil
	}

	return nil, fmt.Errorf("unexpected implementation response: %s", string(raw))
}

func (c *jsonrpcClient) References(ctx context.Context, params ReferenceParams) ([]Location, error) {
	var locs []Location
	var raw json.RawMessage
	if err := c.call(ctx, "textDocument/references", params, &raw); err != nil {
		return nil, err
	}

	if string(raw) == "null" {
		return nil, nil
	}

	if err := json.Unmarshal(raw, &locs); err == nil {
		return locs, nil
	}

	return nil, fmt.Errorf("unexpected references response: %s", string(raw))
}

func (c *jsonrpcClient) Hover(ctx context.Context, params HoverParams) (*Hover, error) {
	var hover Hover
	if err := c.call(ctx, "textDocument/hover", params, &hover); err != nil {
		return nil, err
	}
	return &hover, nil
}

func (c *jsonrpcClient) PrepareCallHierarchy(ctx context.Context, params CallHierarchyPrepareParams) ([]CallHierarchyItem, error) {
	var items []CallHierarchyItem
	if err := c.call(ctx, "textDocument/prepareCallHierarchy", params, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *jsonrpcClient) IncomingCalls(ctx context.Context, params CallHierarchyIncomingCallsParams) ([]CallHierarchyIncomingCall, error) {
	var calls []CallHierarchyIncomingCall
	if err := c.call(ctx, "callHierarchy/incomingCalls", params, &calls); err != nil {
		return nil, err
	}
	return calls, nil
}

func (c *jsonrpcClient) OutgoingCalls(ctx context.Context, params CallHierarchyOutgoingCallsParams) ([]CallHierarchyOutgoingCall, error) {
	var calls []CallHierarchyOutgoingCall
	if err := c.call(ctx, "callHierarchy/outgoingCalls", params, &calls); err != nil {
		return nil, err
	}
	return calls, nil
}

func (c *jsonrpcClient) Close() error {
	// Send shutdown/exit if possible
	ctx := context.Background()
	c.call(ctx, "shutdown", nil, nil)
	// Notification exit
	exitReq := JSONRPCRequest{JSONRPC: "2.0", Method: "exit"}
	exitData, _ := json.Marshal(exitReq)
	fmt.Fprintf(c.writer, "Content-Length: %d\r\n\r\n%s", len(exitData), exitData)

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
