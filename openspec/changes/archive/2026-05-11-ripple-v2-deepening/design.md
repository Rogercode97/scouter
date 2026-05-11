# Design: Ripple V2 Deepening (Cross-Package Type Resolution)

## Technical Approach

We will transition Scouter from heuristic string-based interface matching to semantic type resolution using `go/types`. This involves:
1. Extending the symbol schema to include `package_path` and `receiver_type`.
2. Upgrading the Go parser to use `golang.org/x/tools/go/packages` for accurate metadata extraction.
3. Implementing a semantic analyzer that builds a "Global Type Universe" and uses `go/types.Implements` to establish relationships.

## Architecture Decisions

### Decision: Semantic Implementation Discovery
**Choice**: Use `go/types.Implements` during the `analyze` phase by loading the project's type universe with `go/packages`.
**Rationale**: `go/types` is the official source of truth. It correctly handles pointer vs. value receivers, embedding, and cross-package visibility.

### Decision: DB Schema Extension
**Choice**: Add `package_path` and `receiver_type` directly to the `symbols` table.
**Rationale**: Simplicity and performance.

### Decision: Full Type Loading during Analysis
**Choice**: Perform `packages.Load("./...")` with `packages.NeedTypes` during the `ResolveInterfaces` step.
**Rationale**: Correct interface resolution requires a complete view of the type graph.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/store/store.go` | Modify | Update `Symbol` struct and `migrate()` function to include `package_path` and `receiver_type`. |
| `internal/types/types.go` | Modify | Update `Symbol` struct to match store. |
| `internal/engine/parser.go` | Modify | Replace `go/parser` with `go/packages` in `StreamSymbols`. |
| `internal/engine/analyzer.go` | Modify | Rewrite `ResolveInterfaces` using `go/types.Implements`. |
