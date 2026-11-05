package mcp

import (
	"log/slog"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func setupMockServer(db store.Store, logger *slog.Logger) *Server {
	lspMgr := lsp.GetGlobalManager()
	ledger := engine.NewLedger()
	impact := engine.NewImpactEngine(db, lspMgr, nil)
	analyzer := engine.NewAnalysisEngine(db)
	ripple := engine.NewRippleEngine(db, nil, impact)
	ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))
	searchEngine := engine.NewSearchEngine(db, nil, nil)
	diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, lspMgr, searchEngine)
	healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, nil, diagnostic)

	astRules := engine.NewASTRuleEngine(".scouter/rules")
	indexer := engine.NewIndexerPipeline(engine.IndexerConfig{Store: db, Semantic: nil, Analyzer: analyzer, Search: searchEngine, ASTRules: astRules, Logger: logger})

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
		ChronosEngine: chronos,
	}

	return NewServer(opts)
}
