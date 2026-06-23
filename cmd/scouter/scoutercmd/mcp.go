package scoutercmd

import (
	"fmt"
	"os"

	"log/slog"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/display"
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

		semanticEngine := &engine.SemanticEngine{}
		if err := semanticEngine.Init(cmd.Context(), ""); err != nil {
			logger.Warn("Failed to initialize semantic engine", "error", err)
		}

		searchEngine := engine.NewSearchEngine(db, memoryProvider, semanticEngine)
		healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, memoryProvider)
		compact := engine.NewCompactionEngine(db, ledger)
		diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, lspMgr, searchEngine)

		truthEngine := engine.NewTruthEngine(
			db,
			engine.WithMemory(memoryProvider),
			engine.WithAnalyzer(analyzer),
			engine.WithLSP(lspMgr),
			engine.WithImpact(impact),
			engine.WithSearch(searchEngine),
			engine.WithCompact(compact),
			engine.WithHealer(healer),
			engine.WithDiagnostic(diagnostic),
			engine.WithRipple(ripple),
			engine.WithLedger(ledger),
		)

		evolutionEngine := engine.NewEvolutionEngine(db, ledger, ripple)
		appService := memory.NewAppService(memoryProvider)
		chronos := engine.NewChronosEngine()

		opts := mcp.Options{
			Store:         db,
			Logger:        logger,
			LspMgr:        lspMgr,
			Indexer:       truthEngine,
			Discovery:     truthEngine,
			Intelligence:  truthEngine,
			Evolution:     evolutionEngine,
			Healer:        truthEngine,
			ChronosEngine: chronos,
			AppService:    appService,
			Presenter:     display.NewDefaultPresenter(),
		}

		server := mcp.NewServer(opts)
		defer server.Close()

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
