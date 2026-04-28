package mcp

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official MCP SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer *mcp.Server
	store     store.Repository
	resolver  *PointerResolver
	logger    *slog.Logger
	mu        sync.Mutex
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
		Name:        "index",
		Description: "Index a file or directory for AST symbols",
	}, s.handleIndex)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search",
		Description: "Search for symbols using semantic or text search",
	}, s.handleSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "read",
		Description: "Read a specific symbol or fragment from a file",
	}, s.handleRead)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "callers",
		Description: "Find all callers of a given function or method",
	}, s.handleCallers)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "impact",
		Description: "Calculate the impact (blast radius) of changing a symbol",
	}, s.handleImpact)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "critical_code",
		Description: "Identify high-risk symbols (high centrality and fragility)",
	}, s.handleCritical)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "dependencies",
		Description: "Get a map of all project dependencies",
	}, s.handleDependencies)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "structural_search",
		Description: "Search for code using structural patterns (ast-grep style)",
	}, s.handleStructuralSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "pure_signal",
		Description: "Extract Pure Signal from text using RTK synergy",
	}, s.handlePureSignal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "obsidian_export",
		Description: "Export impact analysis as an Obsidian-ready markdown note",
	}, s.handleObsidianExport)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "hybrid_search",
		Description: "Unify AST symbols with Engram technical wisdom for context-aware search",
	}, s.handleHybridSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "save_anchor",
		Description: "Save a technical session summary directly into Engram memory",
	}, s.handleSaveAnchor)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "self_heal",
		Description: "Execute an autonomous RCA -> Fix -> Verify loop for Go test failures",
	}, s.handleSelfHeal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ripple_refactor",
		Description: "Propagate architectural changes (rename, signature change) across the entire codebase",
	}, s.handleRippleRefactor)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "evolve",
		Description: "Apply a multi-file architectural evolution proposal with atomic rollback and safe evaluation",
	}, s.handleEvolve)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "predict",
		Description: "Identify tests affected by current changes using the Global Call Graph",
	}, s.handlePredict)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "compact_context",
		Description: "Trigger a self-summarization loop to reduce context window noise",
	}, s.handleCompactContext)
}
