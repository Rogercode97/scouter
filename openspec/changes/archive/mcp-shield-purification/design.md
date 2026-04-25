# 🛡️ Technical Design: Blindaje y Purificación de MCP (Phase 1)

## 1. Technical Approach (The Sovereign Pivot)
Scouter will integrate the official `github.com/modelcontextprotocol/go-sdk/mcp` (v1.5.0). This ensures 100% specification compliance and eliminates architectural debt. We will migrate the current manual server loop to use the SDK's high-level abstractions for tools, resources, and prompts registration, while maintaining strict isolation of I/O streams.

## 2. Architecture Decisions

### Decision 1: Official SDK Integration
- **Choice**: Use `github.com/modelcontextprotocol/go-sdk/mcp/server` and `github.com/modelcontextprotocol/go-sdk/mcp/transport/stdio`.
- **Rationale**: The official SDK handles framing, concurrency, and JSON-RPC 2.0 natively. It resolves the "Stdout hijacking" risk by providing a dedicated Stdio transport that explicitly manages `os.Stdin` and `os.Stdout` without global redirection.

### Decision 2: Explicit Dependency Injection
- **Choice**: The Scouter `Server` will act as a wrapper that initializes the official `mcp.Server` with a `stdio.NewTransport()` and an explicit `*slog.Logger`.
- **Rationale**: Ensures that the `internal/mcp` package remains deterministic and testable. Side effects are contained within the `main.go` entrypoint wiring.

### Decision 3: Functional Handler Mapping
- **Choice**: Map Scouter's existing logic to SDK Tools. Each tool (e.g., `scouter_search`) will be registered using `server.AddTool`.
- **Rationale**: Leverages the SDK's built-in validation and type safety for MCP tool definitions.

## 3. Data Flow (SDK-Powered)
```ascii
[Client] --> Stdin --> [SDK Stdio Transport] --> [SDK JSON-RPC Dispatcher]
                                                         |
                                                         v
                                                 [Scouter Tool Handler]
                                                         |
                                                         v
[Client] <-- Stdout <-- [SDK Stdio Transport] <-- [SDK Response Formatter]
```

## 4. File Changes (Impact-Verified)
| File | Action | Rationale |
|------|--------|-----------|
| `go.mod` | **MODIFY** | Add `github.com/modelcontextprotocol/go-sdk`. |
| `internal/mcp/server.go` | **REFACTOR** | Replaces manual loop with SDK `mcp.NewServer` and `server.Serve`. |
| `internal/mcp/handlers.go` | **REFACTOR** | Adapts Scouter tool logic to the `mcp.Tool` interface. |
| `internal/mcp/server_test.go` | **CREATE** | Uses SDK's testing primitives to achieve 100% coverage. |
| `cmd/scouter/main.go` | **MODIFY** | Wires the SDK server and logger to `os.Stderr`. |

## 5. Interfaces / Contracts

```go
package mcp

import (
	"context"
	"log/slog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
}

// NewServer initializes a sovereign, SDK-based MCP server.
func NewServer(name, version string, logger *slog.Logger) *Server

// Start launches the server using the provided transport.
func (s *Server) Start(ctx context.Context, transport mcp.Transport) error
```
