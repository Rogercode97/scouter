# Technical Design: Semantic Purification (v4.0)

## Technical Approach
Inject a stateless, concurrent-safe `SourceResolver` into `ApplyPipeline` to decouple I/O operations from filter execution, standardizing pure semantic analysis across the engine.

## Architecture Decisions

### 1. Concrete SourceResolver Injection
- **Choice**: Implement a `LocalFileResolver` in the `engine` package and inject it during `ApplyPipeline` execution.
- **Alternatives**: Hardcoding file reading within individual action handlers or using global state.
- **Rationale**: Dependency injection via an interface (`SourceResolver`) guarantees testability (allowing mock resolvers) and adheres to Ports and Adapters.

### 2. Concurrency Safety Model
- **Choice**: Stateless implementation relying strictly on `os.ReadFile`.
- **Alternatives**: Implementing an in-memory caching layer with `sync.RWMutex`.
- **Rationale**: `os.ReadFile` is natively safe for concurrent use. A stateless resolver avoids race conditions in multi-threaded pipeline executions without the overhead of lock contention.

### 3. Error Handling and Empty State Policy
- **Choice**: Propagate I/O errors explicitly and handle empty files via a dedicated sentinel error (`ErrEmptySource`).
- **Alternatives**: Silent skipping or returning `nil` slices.
- **Rationale**: Explicit error propagation allows the pipeline supervisor to log accurate telemetry. Treating empty sources as a domain error rather than an I/O fault prevents pipeline halting while signaling that purification is impossible.

## Data Flow

```ascii
[ Pipeline Supervisor ]
          |
          | (1) ApplyPipeline(ctx, config, resolver)
          v
+-----------------------+
| internal/engine       |
| ApplyPipeline()       |
+-----------------------+
          |
          | (2) Context injection
          v
+-----------------------+           (3) Resolve(path)            +-----------------------+
| internal/filter       | -------------------------------------> | internal/engine       |
| Action Handler        |                                        | LocalFileResolver     |
|                       | <------------------------------------- | (os.ReadFile)         |
+-----------------------+           (4) []byte, error            +-----------------------+
          |
          | (5) Semantic Analysis
          v
[ AST Processing Layer ]
```

## File Changes (Impact-Verified)

| File | Action | Rationale |
|------|--------|-----------|
| `internal/engine/pipeline.go` | Modify | Update `ApplyPipeline` signature to accept `SourceResolver` and inject it into the pipeline execution context. |
| `internal/engine/resolver.go` | Add | Implement `LocalFileResolver` struct fulfilling the `SourceResolver` interface. |
| `internal/filter/actions.go` | Modify | Refactor actions to utilize the injected `SourceResolver` for all file retrieval, removing direct `os` imports. |

## Interfaces / Contracts

```go
package engine

import "errors"

// ErrEmptySource indicates the resolved file contains no data.
var ErrEmptySource = errors.New("source file is empty")

// SourceResolver defines the contract for fetching source material safely.
type SourceResolver interface {
	Resolve(path string) ([]byte, error)
}

// LocalFileResolver implements SourceResolver for local filesystem reads.
// It is fully stateless and concurrent-safe.
type LocalFileResolver struct{}

// Resolve reads the file at the given path using os.ReadFile.
func (r *LocalFileResolver) Resolve(path string) ([]byte, error) {
    // 1. Execute os.ReadFile
    // 2. Return standard error if I/O fails
    // 3. Return ErrEmptySource if len == 0
}
```
