package mcp

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func setupMockServer(db store.Store, logger *slog.Logger) *Server {
	engramPath := filepath.Join(os.TempDir(), "scouter_mock_engram.db")
	memoryProvider := engram.NewSQLiteMemoryProvider(engramPath)

	lspMgr := lsp.GetGlobalManager()
	ledger := engine.NewLedger()
	impact := engine.NewImpactEngine(db, lspMgr, memoryProvider)
	analyzer := engine.NewAnalysisEngine(db)
	ripple := engine.NewRippleEngine(db, nil, impact)
	ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))
	searchEngine := engine.NewSearchEngine(db, memoryProvider, nil)
	diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, lspMgr, searchEngine)
	healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, memoryProvider, diagnostic)

	astRules := engine.NewASTRuleEngine(".scouter/rules")
	indexer := engine.NewIndexerPipeline(engine.IndexerConfig{Store: db, Semantic: nil, Analyzer: analyzer, Search: searchEngine, ASTRules: astRules, Logger: logger})

	appService := memory.NewAppService(memoryProvider)
	chronos := engine.NewChronosEngine()
	evolutionEngine := engine.NewEvolutionEngine(db, ledger, ripple)

	opts := Options{
		Store:         db,
		Logger:        logger,
		LspMgr:        lspMgr,
		Indexer:       indexer,
		Search:        searchEngine,
		Analyzer:      analyzer,
		Impact:        impact,
		Diagnostic:    diagnostic,
		ASTRules:      astRules,
		Evolution:     evolutionEngine,
		Healer:        healer,
		Memory:        memoryProvider,
		ChronosEngine: chronos,
		AppService:    appService,
	}

	return NewServer(opts)
}
