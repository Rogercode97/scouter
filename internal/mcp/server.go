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

const MaxSnapshots = 20

// Server wraps the official MCP SDK server to provide Scouter-specific domain logic.
type Server struct {
	mcpServer       *mcp.Server
	store           store.Store
	resolver        *PointerResolver
	lspMgr          *lsp.Manager
	engine          *engine.TruthEngine
	appService      *memory.AppService
	logger          *slog.Logger
	mu              sync.RWMutex
	sessionHistory  []memory.Message
	arsenalUnlocked bool
	chronos         *engine.ChronosEngine
	snapshotsMu     sync.RWMutex
	snapshots       map[string]*engine.ChronosSnapshot
	snapshotOrder   []string // insertion order for LRU eviction
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
		lspMgr:    lsp.NewManager(),
		logger:    logger,
		chronos:   engine.NewChronosEngine(),
		snapshots: make(map[string]*engine.ChronosSnapshot),
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
	search := engine.NewSearchEngine(st, memoryProvider)
	healer := engine.NewHealerEngine(st, s.lspMgr, analyzer, impact, search)
	compact := engine.NewCompactionEngine(st, ledger)
	diagnostic := engine.NewDiagnosticEngine(st, analyzer, impact, healer, s.lspMgr)
	sdd := engine.NewSDDEngine(".")

	s.engine = engine.NewTruthEngine(
		st,
		engine.WithMemory(memoryProvider),
		engine.WithAnalyzer(analyzer),
		engine.WithLSP(s.lspMgr),
		engine.WithImpact(impact),
		engine.WithSearch(search),
		engine.WithCompact(compact),
		engine.WithHealer(healer),
		engine.WithDiagnostic(diagnostic),
		engine.WithRipple(ripple),
		engine.WithSDD(sdd),
		engine.WithLedger(ledger),
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
		Name:        "scouter_heal",
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
		Name:        "cognitive_compact",
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
		Name:        "ast_map",
		Description: "MANDATORY BEFORE EDITING. Map a file or directory to return its skeleton (signatures without bodies) to minimize context noise.",
	}, s.handleMap)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_index",
		Description: "Index a file or directory for AST symbols. Required for deep structural navigation.",
	}, s.handleIndex)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_snapshot",
		Description: "SAFEGUARD: Take an AST snapshot before editing to guarantee structural integrity and prevent symbol drift.",
	}, s.handleSnapshotAST)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_verify",
		Description: "POST-EDIT VALIDATION: Verify against a snapshot to detect missing or mangled symbols.",
	}, s.handleVerifyAST)


	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_search",
		Description: "Semantic or text search for symbols. Use to locate points of interest within a codebase.",
	}, s.handleSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_read",
		Description: "Read a specific symbol or fragment. Use for surgical inspection instead of reading entire files.",
	}, s.handleRead)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_callers",
		Description: "Trace call hierarchy. Mandatory for understanding symbol dependencies before refactoring.",
	}, s.handleCallers)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_definition",
		Description: "LSP-powered Go-To-Definition. Essential for navigating complex dependency chains.",
	}, s.handleGotoDefinition)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_type_info",
		Description: "Get type info and hover docs. Use to clarify interface implementations or complex types.",
	}, s.handleTypeInfo)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "risk_impact",
		Description: "CRITICAL: Calculate blast radius of a change. Mandatory before any staged commit.",
	}, s.handleImpact)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_unlock",
		Description: "Elevate privileges to access specialized architectural and healing engines.",
	}, s.handleUnlockArsenal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ledger_commit",
		Description: "FINALITY: Atomic commit of all staged changes in the Ledger to disk.",
	}, s.handleCommit)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ledger_rollback",
		Description: "EMERGENCY: Clear all staged changes and revert the Ledger to a clean state.",
	}, s.handleRollback)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ledger_diff",
		Description: "Review staged changes and budget status before final commitment.",
	}, s.handleDiff)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_diagnose",
		Description: "PROACTIVE RCA: Generate a Diagnostic HUD (ZON format) from error logs with Thermal/X-Ray vision.",
	}, s.handleDiagnose)
}

func (s *Server) registerSpecializedTools() {
	trueVal := true

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "risk_critical_code",
		Description: "Identify high-risk symbols (high centrality and fragility) that require extra architectural scrutiny.",
	}, s.handleCritical)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ast_structural_search",
		Description: "Advanced structural search using patterns (ast-grep style). Ideal for finding complex refactoring targets.",
	}, s.handleStructuralSearch)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cognitive_signal",
		Description: "NOISE FILTER: Extract Pure Signal from text using RTK synergy for maximum clarity and token efficiency.",
	}, s.handlePureSignal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cognitive_obsidian",
		Description: "KNOWLEDGE CAPTURE: Export impact analysis as an Obsidian-ready markdown note for long-term lore persistence.",
	}, s.handleObsidianExport)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cognitive_anchor",
		Description: "SESSION PERSISTENCE: Save a technical summary directly into Engram memory to maintain cross-session context.",
	}, s.handleSaveAnchor)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scouter_heal",
		Description: "AUTONOMOUS HEALING: Execute a full RCA -> Fix -> Verify loop for Go test failures. High-fidelity repair engine.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleSelfHeal)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ledger_ripple",
		Description: "ARCHITECTURAL PROPAGATION: Rename or change signatures across the entire codebase atomically with Ripple Engine.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleRippleRefactor)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ledger_evolve",
		Description: "SAFE EVOLUTION: Apply multi-file architectural changes with atomic rollback and high-fidelity evaluation.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &trueVal},
	}, s.handleEvolve)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "risk_predict",
		Description: "INTELLIGENT TESTING: Identify tests affected by current changes using the Global Call Graph.",
	}, s.handlePredict)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "cognitive_dream",
		Description: "ARCHITECTURAL ALIGNMENT: Distill ADRs and Pattern summaries from project history memory.",
	}, s.handleDream)
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
