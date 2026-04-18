# Proposal: Wave 8 Final Purge

## Intent

Achieve absolute compliance with `go-divine` and `mcp-sovereign` standards (Rating 10/10) by addressing the remaining architectural flaws discovered during the second Judgment Day audit. This change fixes critical OOM vulnerabilities, ensures 100% context isolation in database initialization, and introduces strong Glasswall validation boundaries for MCP inputs.

## Scope

### In Scope
- Add a strict `LIMIT 100` (or configurable parameter) to FTS5 queries in `SearchSymbols` to prevent out-of-memory (OOM) crashes on large codebases.
- Refactor `store.New` to accept `context.Context` and return the `Repository` interface instead of the concrete `*Store`.
- Implement Glasswall Validation by creating explicit struct types with `validate` tags for MCP tool arguments (e.g., `ScouterReadRequest`) in `cmd/scouter/main.go`.

### Out of Scope
- Major feature additions like Call Graph Indexing (reserved for V2.0).
- Replacing SQLite with a different database engine.

## Capabilities

### New Capabilities
- `mcp-validation`: Strict validation structs for MCP input boundaries.

### Modified Capabilities
- `symbol-search`: Search behavior is capped to prevent OOM.
- `store-initialization`: Initialization is now cancelable via Context.

## Approach

1. **Kata del Límite**: Update the SQL strings in `internal/store/store.go` (`SearchSymbols` method) to include `LIMIT 100`.
2. **Kata de Inicialización**: Change `store.New` signature to `func New(ctx context.Context, dbPath string) (Repository, error)`. Update `cmd/scouter/main.go` and tests to pass `ctx` and handle the interface.
3. **Kata del Cristal**: Add `github.com/go-playground/validator/v10`. Define `IndexRequest`, `SearchRequest`, and `ReadRequest` structs with tags like `validate:"required"` in `cmd/scouter/main.go` (or a dedicated package). Unmarshal MCP arguments into these structs and call `validator.Struct()` before passing them to the engine.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/store/store.go` | Modified | Search query limit and New() constructor signature. |
| `cmd/scouter/main.go` | Modified | Store initialization and MCP tool handler argument parsing. |
| `cmd/index-vault/main.go` | Modified | Store initialization. |
| `internal/store/store_test.go` | Modified | Store initialization in tests. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Database initialization blocks context | Low | Use `Context` appropriately in `sql.Open` if supported by driver, or in pinging. The primary fix is propagating the context to allow upper layers to cancel. |
| MCP tools fail due to rigid validation | Medium | Ensure validation tags exactly match the expected JSON structure from clients (Gemini CLI, OpenCode). Test all tools after adding `validator/v10`. |

## Rollback Plan

Revert the specific commit containing the `wave8-final-purge` changes. No schema migrations are required, making the rollback purely code-based.

## Dependencies

- Requires adding `github.com/go-playground/validator/v10` dependency to `go.mod`.

## Success Criteria

- [ ] `store.New` returns `store.Repository`.
- [ ] `SearchSymbols` successfully caps results at 100.
- [ ] Invalid MCP arguments are rejected by `validator` before calling the engine.
- [ ] Project compiles and all unit tests pass (`go test ./...`).