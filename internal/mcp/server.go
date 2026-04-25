package mcp

import (
	"context"
	"log/slog"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official MCP SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer *mcp.Server
	store     store.Repository
	resolver  *PointerResolver
	logger    *slog.Logger
}

// NewServer initializes a sovereign, SDK-based MCP server.
func NewServer(st store.Repository, logger *slog.Logger) *Server {
	implementation := &mcp.Implementation{
		Name:    "scouter",
		Version: "2.6.2-sovereign",
	}
	
	s := &Server{
		mcpServer: mcp.NewServer(implementation, &mcp.ServerOptions{
			Logger: logger,
		}),
		store:    st,
		resolver: NewPointerResolver(st),
		logger:   logger,
	}

	s.registerTools()
	return s
}

// Start launches the server using the provided transport.
func (s *Server) Start(ctx context.Context, transport mcp.Transport) error {
	return s.mcpServer.Run(ctx, transport)
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_index",
		Description: "Index a file or directory for AST symbols",
	}, s.handleIndex)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_search",
		Description: "Search for symbols using semantic or text search",
	}, s.handleSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_read",
		Description: "Read a specific symbol or fragment from a file",
	}, s.handleRead)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_callers",
		Description: "Find all callers of a given function or method",
	}, s.handleCallers)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_impact",
		Description: "Calculate the impact (blast radius) of changing a symbol",
	}, s.handleImpact)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_critical_code",
		Description: "Identify high-risk symbols (high centrality and fragility)",
	}, s.handleCritical)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_dependencies",
		Description: "Get a map of all project dependencies",
	}, s.handleDependencies)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_structural_search",
		Description: "Search for code using structural patterns (ast-grep style)",
	}, s.handleStructuralSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_pure_signal",
		Description: "Extract Pure Signal from text using RTK synergy",
	}, s.handlePureSignal)
}
