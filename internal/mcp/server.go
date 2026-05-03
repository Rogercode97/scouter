package mcp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official MCP SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer  *mcp.Server
	store      store.Repository
	resolver   *PointerResolver
	lspMgr     *lsp.Manager
	engine     *engine.TruthEngine
	appService *memory.AppService
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewServer initializes a sovereign, SDK-based MCP server.
func NewServer(st store.Repository, logger *slog.Logger) *Server {
	implementation := &mcp.Implementation{
		Name:    "scouter",
		Version: "12.0.0-ascension",
	}
	
	s := &Server{
		mcpServer: mcp.NewServer(implementation, &mcp.ServerOptions{
			Logger: logger,
		}),
		store:    st,
		resolver: NewPointerResolver(st),
		lspMgr:   lsp.NewManager(),
		logger:   logger,
	}

		// [Sovereignty Upgrade] Initialize Engines
	impact := engine.NewImpactEngine(st, s.lspMgr)
	analyzer := engine.NewAnalysisEngine(st)
	ripple := engine.NewRippleEngine(st, nil, impact)
	healer := engine.NewHealerEngine(st, s.lspMgr, analyzer, impact)
	search := engine.NewSearchEngine(st)
	compact := engine.NewCompactionEngine(st)

	s.engine = engine.NewTruthEngine(
		st,
		analyzer,
		s.lspMgr,
		impact,
		search,
		compact,
		healer,
		ripple,
		nil, // Messenger will be injected per-request if needed
	)

	// [Dream Ascension] Initialize Memory Service
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "scouter", "scouter.db")
	memoryProvider := engram.NewSQLiteMemoryProvider(dbPath)
	repo := engram.NewEngramRepository(false)
	s.appService = memory.NewAppService(memoryProvider, nil, repo)

	s.registerTools()
	s.registerResources()
	return s
}

// Start launches the server using the provided transport.
func (s *Server) Start(ctx context.Context, transport mcp.Transport) error {
	return s.mcpServer.Run(ctx, transport)
}

// Close gracefully shuts down the server and its resources.
func (s *Server) Close() error {
	s.logger.Info("shutting down MCP server")
	return s.lspMgr.Close()
}

func (s *Server) registerTools() {
	trueVal := true

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
		Name:        "goto_definition",
		Description: "Find the definition of the symbol at the given position using LSP",
	}, s.handleGotoDefinition)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "type_info",
		Description: "Get type information and hover documentation for the symbol at the given position",
	}, s.handleTypeInfo)

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
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleSelfHeal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ripple_refactor",
		Description: "Propagate architectural changes (rename, signature change) across the entire codebase",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleRippleRefactor)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "evolve",
		Description: "Apply a multi-file architectural evolution proposal with atomic rollback and safe evaluation",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleEvolve)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "predict",
		Description: "Identify tests affected by current changes using the Global Call Graph",
	}, s.handlePredict)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "compact_context",
		Description: "Trigger a self-summarization loop to reduce context window noise",
	}, s.handleCompactContext)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_judge",
		Description: "Execute an adversarial review of a code change or proposal using parallel sampling",
	}, s.handleJudge)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "dream",
		Description: "Runs the memory distillation loop for a project to generate ADRs and Pattern summaries",
	}, s.handleDream)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "knowledge_graph",
		Description: "Queries Engram for ADRs that specifically mention or affect the given AST symbol",
	}, s.handleKnowledgeGraph)
}
