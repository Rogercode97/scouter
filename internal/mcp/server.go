package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const MaxSessionHistory = 100

// Server wraps the official MCP SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer      *mcp.Server
	store          store.Store
	resolver       *PointerResolver
	lspMgr         *lsp.Manager
	engine         *engine.TruthEngine
	appService     *memory.AppService
	logger         *slog.Logger
	mu             sync.RWMutex
	sessionHistory []memory.Message
	arsenalUnlocked bool
}

// NewServer initializes a sovereign, SDK-based MCP server.
func NewServer(st store.Store, logger *slog.Logger) *Server {
	implementation := &mcp.Implementation{
		Name:    "scouter",
		Version: "12.0.0-ascension",
	}

	opts := &mcp.ServerOptions{
		Logger: logger,
		Instructions: ScouterServerInstructions,
	}
	
	s := &Server{
		mcpServer: mcp.NewServer(implementation, opts),
		store:    st,
		resolver: NewPointerResolver(st),
		lspMgr:   lsp.NewManager(),
		logger:   logger,
	}

	// [Dream Ascension] Initialize Memory Service
	engramPath, _ := engram.DiscoverDBPath()
	memoryProvider := engram.NewSQLiteMemoryProvider(engramPath)

	// [Sovereignty Upgrade] Initialize Engines
	ledger := engine.NewLedger() // Staging Ledger with persistence
	impact := engine.NewImpactEngine(st, s.lspMgr, memoryProvider)
	analyzer := engine.NewAnalysisEngine(st)
	ripple := engine.NewRippleEngine(st, nil, impact)
	ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))
	healer := engine.NewHealerEngine(st, s.lspMgr, analyzer, impact)
	search := engine.NewSearchEngine(st, memoryProvider)
	compact := engine.NewCompactionEngine(st, ledger)
	diagnostic := engine.NewDiagnosticEngine(st, analyzer, impact, healer, s.lspMgr)
	sdd := engine.NewSDDEngine(".")

	s.engine = engine.NewTruthEngine(
		st,
		memoryProvider,
		analyzer,
		s.lspMgr,
		impact,
		search,
		compact,
		healer,
		diagnostic,
		ripple,
		sdd,
		ledger,
		nil, // astRules
		nil, // Messenger will be injected per-request if needed
	)

	s.appService = memory.NewAppService(memoryProvider, nil)

	s.registerCoreTools()
	s.registerResources()
	s.registerPrompts()
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

// AppendSessionMessage adds a message to the session history with a fixed capacity.
func (s *Server) AppendSessionMessage(msg memory.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessionHistory) >= MaxSessionHistory {
		// Evict oldest (ring buffer behavior)
		s.sessionHistory = append(s.sessionHistory[1:], msg)
	} else {
		s.sessionHistory = append(s.sessionHistory, msg)
	}
}

// GotoDefinition performs an LSP definition request.
func (s *Server) GotoDefinition(ctx context.Context, path string, pos lsp.Position) ([]lsp.Location, error) {
	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return nil, err
	}
	return client.Definition(ctx, lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + path},
			Position:     pos,
		},
	})
}

// Hover performs an LSP hover request.
func (s *Server) Hover(ctx context.Context, path string, pos lsp.Position) (*lsp.Hover, error) {
	client, err := s.lspMgr.GetClient(ctx, path)
	if err != nil {
		return nil, err
	}
	return client.Hover(ctx, lsp.HoverParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + path},
			Position:     pos,
		},
	})
}

// --- Shared Helpers ---

// mcpMessenger adapts MCP Sampling to TruthEngine's Messenger interface.
type mcpMessenger struct {
	server *Server
	req    *mcp.CallToolRequest
}

func (m *mcpMessenger) Ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Log user prompt
	m.server.AppendSessionMessage(memory.Message{Role: "user", Content: userPrompt})

	res, err := m.req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: userPrompt}},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return "", err
	}
	txt, ok := res.Content.(*mcp.TextContent)
	if !ok {
		return "", fmt.Errorf("unexpected sampling response type")
	}

	// Log assistant response
	m.server.AppendSessionMessage(memory.Message{Role: "assistant", Content: txt.Text})

	return txt.Text, nil
}

// healerMessenger adapts MCP Sampling with Engram context for the Healer engine.
type healerMessenger struct {
	server    *Server
	req       *mcp.CallToolRequest
	engramCtx string
}

func (m *healerMessenger) Ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	fullUserPrompt := fmt.Sprintf("%s\n\nHistorical Context (Engram):\n%s", userPrompt, m.engramCtx)
	
	// Log user prompt
	m.server.AppendSessionMessage(memory.Message{Role: "user", Content: fullUserPrompt})

	res, err := m.req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: fullUserPrompt}},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return "", err
	}
	txt, ok := res.Content.(*mcp.TextContent)
	if !ok {
		return "", fmt.Errorf("unexpected sampling response type")
	}

	// Log assistant response
	m.server.AppendSessionMessage(memory.Message{Role: "assistant", Content: txt.Text})

	return txt.Text, nil
}

func (s *Server) fetchEngramContext(ctx context.Context, query string) string {
	insights, err := s.engine.MemoryProvider().SearchInsights(ctx, query, 3)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	for _, in := range insights {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", in.Type, in.Title, in.Why))
	}

	res := sb.String()
	if len(res) > 1000 {
		return res[:1000] + "\n...[truncated]"
	}
	return res
}

func (s *Server) registerPrompts() {
	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "self_heal",
		Description: "System prompt for the Atomic Self-Healing engine",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "System prompt for the Atomic Self-Healing engine",
			Messages: []*mcp.PromptMessage{
				{
					Role: mcp.Role("assistant"),
					Content: &mcp.TextContent{
						Text: SelfHealSystemPrompt,
					},
				},
			},
		}, nil
	})

	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "gep_mutator",
		Description: "System prompt for the Genome Mutator (GEP)",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "System prompt for the Genome Mutator (GEP)",
			Messages: []*mcp.PromptMessage{
				{
					Role: mcp.Role("assistant"),
					Content: &mcp.TextContent{
						Text: GEPSystemPrompt,
					},
				},
			},
		}, nil
	})

	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "compact_context",
		Description: "System prompt for the Compaction Engine",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "System prompt for the Compaction Engine",
			Messages: []*mcp.PromptMessage{
				{
					Role: mcp.Role("assistant"),
					Content: &mcp.TextContent{
						Text: CompactContextSystemPrompt,
					},
				},
			},
		}, nil
	})

	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "judge",
		Description: "System prompt for the Cynical Adversarial Judge",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "System prompt for the Cynical Adversarial Judge",
			Messages: []*mcp.PromptMessage{
				{
					Role: mcp.Role("assistant"),
					Content: &mcp.TextContent{
						Text: JudgeSystemPrompt,
					},
				},
			},
		}, nil
	})
}

func (s *Server) registerCoreTools() {
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
		Name:        "unlock_heavy_arsenal",
		Description: "Unlock specialized architectural evolution and healing tools",
	}, s.handleUnlockArsenal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_commit",
		Description: "Commit all staged changes in the Ledger to disk",
	}, s.handleCommit)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_rollback",
		Description: "Rollback and clear all staged changes in the Ledger",
	}, s.handleRollback)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_diff",
		Description: "Preview all staged changes and mission budget status",
	}, s.handleDiff)
}

func (s *Server) registerSpecializedTools() {
	trueVal := true

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "critical_code",
		Description: "Identify high-risk symbols (high centrality and fragility)",
	}, s.handleCritical)

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
		Name:        "find_logical_twin",
		Description: "Find structurally identical symbols across the codebase",
	}, s.handleFindLogicalTwin)

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

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "explore_sdd",
		Description: "Search and explore SDD artifacts (roadmap, tasks, specs)",
	}, s.handleExploreSDD)
}

type UnlockArsenalParams struct{}

func (s *Server) handleUnlockArsenal(ctx context.Context, req *mcp.CallToolRequest, args UnlockArsenalParams) (*mcp.CallToolResult, any, error) {
        s.mu.Lock()
        defer s.mu.Unlock()

        if s.arsenalUnlocked {
                return &mcp.CallToolResult{
                        Content: []mcp.Content{
                                &mcp.TextContent{Text: "Arsenal is already unlocked."},
                        },
                }, nil, nil
        }

        s.registerSpecializedTools()
        s.arsenalUnlocked = true

        return &mcp.CallToolResult{
                Content: []mcp.Content{
                        &mcp.TextContent{Text: "Specialized Arsenal unlocked. New tools are now available for discovery."},
                },
        }, nil, nil
}


var execCommand = exec.Command
