# Scouter Engine Architecture

This document explains the architectural principles governing the core engines and the MCP server within the Scouter project.

## Dependency Injection (DI)

To maintain a decoupled and highly testable architecture, Scouter avoids global state and hardcoded dependencies within its core services.

The `mcp.Server` is instantiated via manual Dependency Injection using an `Options` struct. This approach provides clean inversion of control without the overhead of heavy DI frameworks like Uber `fx` or Google `wire`.

```go
type Options struct {
        Store        store.Store
        Logger       *slog.Logger
        LspMgr       *lsp.Manager
        Indexer      engine.IndexerService
        Discovery    engine.DiscoveryService
        Intelligence engine.IntelligenceService
        Evolution    engine.EvolutionService
        Healer       engine.HealerService
        Memory       memory.MemoryProvider
        SDD          engine.SDDService
        Presenter    display.Presenter
}
```

The application entrypoint (`cmd/scouter/main.go`) is responsible for wiring these dependencies and injecting them into `mcp.NewServer()`.

## SSA Interface Analysis

Scouter relies on deep Static Single Assignment (SSA) analysis to resolve polymorphic behavior at compile-time.

To correctly map which concrete types implement which interfaces, the engine utilizes `golang.org/x/tools/go/types/typeutil.MethodSetCache`. By iterating through all loaded types in `ssa.Program.AllPackages()` and evaluating them against `types.Implements()`, Scouter generates a robust, cached mapping of interface implementations. This ensures accurate Call Graph generation and dependency tracing, which is vital for the `scouter predict` and `scouter graph` commands.

## Sharding Strategy

The local storage subsystem (`internal/store/sharding.go`) partitions files to ensure performance at scale. It utilizes a `fnv.New64a` hashing algorithm over the full absolute file path to determine the shard key. This prevents "hot shards" that would otherwise occur if multiple heavily-populated files shared the same parent directory hash.
