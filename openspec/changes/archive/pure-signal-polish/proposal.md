# 🛡️ SDD PROPOSAL: Pure Signal Polish (Pre-V2.0)

## 🎯 1. INTENT (RCA & VALUE)
- **RCA**: `PointerResolver` in `internal/mcp/resolver.go` calculates file hashes redundantly, causing unnecessary I/O overhead. Interface resolution in `internal/store/store.go` lacks method signature matching, leading to low-fidelity dependency mapping. Furthermore, heavy operations in `internal/engine` do not strictly enforce `ctx.Done()`, risking context leakage and violating Context Sovereignty mandates.
- **Value**: Eradicate unnecessary I/O overhead to maximize performance. Achieve absolute precision in the global dependency graph (Interface Resolution) prior to the V2.0 (Sovereignty Edition) launch. Guarantee atomic cancellation and pure context management for large-scale engine operations.

## 🚧 2. SCOPE (ZERO CREEP)
- **IN-SCOPE**:
  - Refactoring `PointerResolver` (`internal/mcp/resolver.go`) to utilize `store.GetFileIndex` for hash retrieval instead of re-calculating it.
  - Modifying the `symbols` schema/storage to include `Signature` for precise method tracking.
  - Updating `ResolveInterfaces` (`internal/store/store.go`) to enforce strict signature matching alongside method names.
  - Injecting strict `ctx.Done()` polling loops in heavy functions within `internal/engine`.
- **OUT-OF-SCOPE**:
  - Full rewrite of the MCP server or the `engine` pipeline.
  - Implementation of the full V2.0 Global Call Graph (this is exclusively a pre-V2.0 purification step).
  - Modifications to `scouter_search` or other MCP capabilities unrelated to the `PointerResolver`.

## ⚔️ 3. CAPABILITIES CONTRACT
- **Modified Capability**: `mcp-file-resolution`
  - *Must resolve pointers using cached file hashes (`file_index`) without triggering additional disk I/O.*
- **Modified Capability**: `store-interface-resolution`
  - *Must match interface implementations using strict `Signature` equality.*
- **Modified Capability**: `engine-context-sovereignty`
  - *Must atomically abort execution immediately when `ctx.Done()` is triggered.*

## 🗺️ 4. AFFECTED AREAS
- `internal/mcp/resolver.go`
- `internal/store/store.go`
- `internal/engine/executor.go`
- `internal/engine/pipeline.go`

## ⏪ 5. ROLLBACK PLAN
- **Trigger**: Integration test failures in `go test ./...` specifically surrounding interface resolution accuracy or context cancellation timeouts.
- **Action**: Execute `git revert <commit-hash>` to rollback changes safely.
- **Validation**: Re-run `go test ./...` to confirm the system returns to its stable Pre-V2.0 baseline.
