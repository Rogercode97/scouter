# 📐 Technical Design: Symbolic Ripple Engine (v5.5)

## 1. Technical Approach
The Symbolic Ripple Engine orchestrates multi-file symbolic refactoring by establishing an atomic transaction scope across dependencies. It utilizes `scouter_callers` to map the exact blast radius of a symbol change and structures modifications within a centralized "Symbolic Ledger" to guarantee true atomicity and zero-slop execution.

## 2. Architecture Decisions

### 2.1 Symbolic Ledger (Pending Modifications)
- **Choice**: Implement an in-memory `DependencyGraphLedger` holding a map of `[FilePath] -> []Patch`.
- **Alternatives**: Immediate sequential writes to disk.
- **Rationale**: Accumulating all patches in-memory prior to disk I/O ensures that if a ripple verification fails at depth N, the file system is never left in a corrupted or half-migrated state.

### 2.2 Call Graph Traversal (`scouter_callers`)
- **Choice**: Breadth-First Search (BFS) over `scouter_callers` to recursively identify dependencies up to `maxDepth`.
- **Alternatives**: Depth-First Search (DFS) or rigid tree mapping.
- **Rationale**: BFS naturally models the "ripple" outwards from the origin kernel, enabling layered batch compilation and structural AST validation checks at each radius ring.

### 2.3 Rollback Strategy (.bak Orchestration)
- **Choice**: Pre-flight `.bak` shadow copying. Before applying the Ledger, all target files are copied to `{filename}.bak`.
- **Alternatives**: Reverting via Git checkout or AST inversion.
- **Rationale**: File-system level backups provide the fastest, most resilient rollback mechanism independent of VCS state, aligning with the Vault's strict recovery mandates.

### 2.4 Multi-File Batching (Atomicity)
- **Choice**: Two-Phase Commit (2PC) protocol (Prepare & Apply).
- **Alternatives**: Streamed rolling updates.
- **Rationale**: Phase 1 validates all `Patch` boundaries against the current AST. Phase 2 synchronously performs the disk flush and shadow copy operations, providing transactional integrity across arbitrary file boundaries.

## 3. Data Flow & The Ripple Effect

```ascii
[Origin Symbol] -----> (scouter_callers BFS Traversal)
       |
       v
+-----------------------+
|   Symbolic Ledger     |
| - fileA.go: [Patch]   |
| - fileB.go: [Patch]   |
+-----------------------+
       |
       | (Two-Phase Commit)
       |
       |--> [Phase 1: Validate AST Boundaries]
       |
       |--> [Phase 2: Generate .bak Copies]
       |
       |--> [Phase 3: Flush Disk I/O]
       |
    [Success] ----> (Purge .bak)
       |
    [Failure] ----> (Restore from .bak & Abort)
```

## 4. File Changes (Impact-Verified)

| File | Action | Rationale |
|------|--------|-----------|
| `internal/engine/ripple.go` | **Create** | Entry point for BFS graph traversal and managing the recursive state. |
| `internal/engine/ledger.go` | **Create** | Core logic for the in-memory `Patch` accumulator and Two-Phase Commit coordinator. |
| `internal/engine/rollback.go` | **Create** | Isolation layer for managing `.bak` file creation, restoration, and purging. |
| `internal/cli/ripple.go` | **Create** | Hooks the Ripple Engine into the CLI, exposing `maxDepth` and `origin` flags. |

## 5. Interfaces / Contracts

```go
package engine

import "context"

// Patch represents a localized structural modification.
type Patch struct {
    TargetSymbol string
    StartByte    int
    EndByte      int
    NewContent   string
}

// SymbolicLedger provides transactional atomicity for multi-file patches.
type SymbolicLedger interface {
    Record(ctx context.Context, filePath string, p Patch) error
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

// RippleEngine defines the propagation boundaries for the blast radius.
type RippleEngine interface {
    Propagate(ctx context.Context, originFile string, symbol string, maxDepth int) error
}
```