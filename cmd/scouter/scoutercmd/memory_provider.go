package scoutercmd

import (
	"log/slog"
	"os"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/memory/engram"
)

// memoryProvider returns the Engram-backed provider used for historical bugfix
// insights, or nil when the observation database cannot be located. The engines
// treat nil as "no history": they still answer, just without the historical
// fragility multiplier. Every surface builds it here so the CLI and the MCP
// server score a symbol the same way.
func memoryProvider() memory.MemoryProvider {
	path, err := engram.DiscoverDBPath()
	if err != nil {
		slog.Debug("engram database not located, historical insights disabled", "error", err)
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		slog.Debug("engram database absent, historical insights disabled", "path", path)
		return nil
	}
	return engram.NewSQLiteMemoryProvider(path)
}
