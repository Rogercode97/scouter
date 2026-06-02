package mcp

import (
	"log/slog"
	
	"github.com/Rogercode97/scouter/internal/adapters/engram"
	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func setupMockServer(db store.Store, logger *slog.Logger) *Server {
	engramPath, _ := engram.DiscoverDBPath()
	memoryProvider := engram.NewSQLiteMemoryProvider(engramPath)

	lspMgr := lsp.GetGlobalManager()
	ledger := engine.NewLedger()
	impact := engine.NewImpactEngine(db, lspMgr, memoryProvider)
	analyzer := engine.NewAnalysisEngine(db)
	ripple := engine.NewRippleEngine(db, nil, impact)
	ripple.Validators = append(ripple.Validators, engine.NewLSPValidator(analyzer.ProjectRoot))
	searchEngine := engine.NewSearchEngine(db, memoryProvider)
	healer := engine.NewHealerEngine(db, lspMgr, analyzer, impact, searchEngine, memoryProvider)
	compact := engine.NewCompactionEngine(db, ledger)
	diagnostic := engine.NewDiagnosticEngine(db, analyzer, impact, healer, lspMgr)
	sdd := engine.NewSDDEngine(".")

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
		engine.WithSDD(sdd),
		engine.WithLedger(ledger),
	)

	appService := memory.NewAppService(memoryProvider, nil)
	chronos := engine.NewChronosEngine()

	opts := Options{
		Store:         db,
		Logger:        logger,
		LspMgr:        lspMgr,
		TruthEngine:   truthEngine,
		ChronosEngine: chronos,
		AppService:    appService,
	}

	return NewServer(opts)
}
