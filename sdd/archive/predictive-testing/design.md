# Design: Predictive Testing

## Architecture
We will use a Recursive CTE in SQLite to traverse the call graph upwards from the modified symbol to find all reachable test functions.

## Components

### 1. Store (internal/store/store.go)
- Add `GetAffectedTests(ctx, symbolID string) ([]string, error)`
- Implementation: `WITH RECURSIVE callers AS (...) SELECT ... WHERE name LIKE 'Test%'`

### 2. Engine (internal/engine/predict.go)
- `Predict(ctx, target string) ([]Symbol, error)`
- Resolves target to Symbol ID.
- Calls `Store.GetAffectedTests`.

### 3. MCP (internal/mcp/handlers.go)
- Tool: `scouter_predict`
- Arguments: `path`, `symbol`

### 4. CLI (internal/cli/)
- Command: `predict`
- Format: `scouter predict <file>[:<symbol>]`
