# Proposal: Ripple V2 Deepening (Cross-Package Type Resolution)

**Intent**:
- Resolve interface implementations across package boundaries using semantic analysis (`go/types`).
- Persist package-aware symbol metadata in the local SQLite graph.
- Increase the reliability of "Ripple" refactors in large, multi-package Go projects.

**Why**: Brittle string matching in the current analyzer causes false negatives/positives during impact analysis, especially across package boundaries.

**Where**: internal/store/store.go, internal/engine/parser.go, internal/engine/analyzer.go

**Scope**:
- **In-Scope**: SQLite schema updates for `package_path` and `receiver_type`; `internal/engine/parser.go` upgrade to `go/packages`; `internal/engine/analyzer.go` refactor for `go/types.Implements` integration.
- **Out-of-Scope**: Support for Go generics (deferred); non-Go language enhancements; TUI/Display layer changes.

**Approach**:
1. **Schema Migration**: Update `internal/store/store.go` to include `package_path` and `receiver_type` (pointer/value flag) for symbols.
2. **Parser Upgrade**: Refactor `internal/engine/parser.go` to use `golang.org/x/tools/go/packages` to load full type information during indexing.
3. **Semantic Analyzer**: Rewrite implementation discovery in `internal/engine/analyzer.go` to use `go/types.Implements` instead of string-based method matching.
4. **Tracer-Bullet Verification**: Create tests with cross-package interface implementations to prove the resolution works.
