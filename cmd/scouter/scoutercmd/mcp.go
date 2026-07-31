package scoutercmd

import (
	"context"
	"fmt"
	"os"

	"log/slog"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Executes mcp",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, closeDB, exitCode := openDB(cmd.Context())
		if exitCode != 0 {
			os.Exit(exitCode)
			return nil
		}
		defer closeDB()

		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

		engramPath, _ := engram.DiscoverDBPath()
		memoryProvider := engram.NewSQLiteMemoryProvider(engramPath)

		lspMgr := lsp.GetGlobalManager()
		ledger := engine.NewLedger()
		impact := engine.NewImpactEngine(db, lspMgr, memoryProvider)
		analyzer := engine.NewAnalysisEngine(db)
		ripple := engine.NewRippleEngine(db, nil, impact)
		ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))

		var semanticEngine engine.Embedder
		se := &engine.SemanticEngine{}
		if err := se.Init(cmd.Context(), ""); err != nil {
			logger.Warn("Failed to initialize semantic engine", "error", err)
		} else {
			semanticEngine = se
		}

		searchEngine := engine.NewSearchEngine(db, memoryProvider, semanticEngine)
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, lspMgr, searchEngine)
		healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, memoryProvider, diagnostic)
		_ = engine.NewCompactionEngine(db, ledger)

		astRules := engine.NewASTRuleEngine(".scouter/rules")
		churnEngine := engine.NewChurnEngine(db)

		evolutionEngine := engine.NewEvolutionEngine(db, ledger, ripple)
		appService := memory.NewAppService(memoryProvider)
		chronos := engine.NewChronosEngine()
		watcher := engine.NewWatcher(logger)

		opts := mcp.Options{
			Store:         db,
			Logger:        logger,
			LspMgr:        lspMgr,
			Indexer:       engine.NewIndexerPipeline(engine.IndexerConfig{Store: db, Semantic: semanticEngine, Analyzer: analyzer, Search: searchEngine, ASTRules: astRules, Churn: churnEngine, Logger: logger}),
			Search:        searchEngine,
			Analyzer:      analyzer,
			Impact:        impact,
			Diagnostic:    diagnostic,
			ASTRules:      astRules,
			Evolution:     evolutionEngine,
			Healer:        healer,
			ChronosEngine: chronos,
			AppService:    appService,
			Watcher:       watcher,
		}

		server := mcp.NewServer(opts)
		defer server.Close()

		if cwd, err := os.Getwd(); err == nil {
			_ = watcher.Start(cmd.Context(), cwd, func(indexCtx context.Context, dir string) error {
				return opts.Indexer.Index(indexCtx, dir)
			})
		}

		transport := &sdk.StdioTransport{}
		if err := server.Start(cmd.Context(), transport); err != nil {
			fmt.Fprintf(os.Stderr, "MCP Server stopped: %v\n", err)
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
