# 🏛️ TASKS: Pure Signal Polish (Pre-V2.0) 👑

## 🔱 PHASE 1: FOUNDATION & PERSISTENCE
- [x] **[DATABASE]** Update `internal/telemetry/schema.go` (or store schema definition) to include the `signature` column in the symbols table.
- [x] **[DATABASE]** Modify `internal/store/store.go` to handle the `signature` field in `Symbol` struct and SQL queries (Insert/Select).
- [x] **[MIGRATION]** Implement/Update the migration logic in `internal/store/store.go` to ensure the `signature` column exists in existing databases.
- [x] **[VERIFICATION]** Run `go test -v internal/store/store_test.go` to verify schema integrity and CRUD operations for the new field.

## 🏹 PHASE 2: CORE SIGNAL EXTRACTION
- [x] **[ENGINE]** Update `internal/engine/parser.go` to extract full signatures (parameters and return types) for functions and methods (Go implementation).
- [x] **[ENGINE]** Map the extracted signature to the `Symbol` struct during the indexing process.
- [ ] **[VERIFICATION]** Create/Update `internal/engine/parser_test.go` with a "Pure Signal" test case verifying signature extraction for Go, TS, and Python.

## ⚔️ PHASE 3: OPTIMIZED RESOLUTION
- [x] **[MCP]** Refactor `internal/mcp/resolver.go` to utilize `GetFileIndex` (from `store`) instead of full table scans where applicable.
- [x] **[STORE]** Refactor `ResolveInterfaces` in `internal/store/store.go` to compare signatures instead of just names or naive heuristic matches.
- [ ] **[LOGGING]** Inject structured logs via `log-sovereign` patterns in `resolver.go` to track resolution latency.

## 🧪 PHASE 4: SUPREME VERIFICATION
- [ ] **[TEST]** Implement `internal/mcp/resolver_perf_test.go` to measure resolution speed improvements using `GetFileIndex`.
- [ ] **[TEST]** Create `tests/integration/interface_fidelity_test.go` to validate complex interface implementations (e.g., matching embedding and method signatures).
- [x] **[AUDIT]** Run `rtk proxy go test ./...` and ensure 0 regressions in signal extraction.

## 📜 SEAL OF SOVEREIGNTY
- **Change**: `pure-signal-polish`
- **Wave**: 8.9
- **Status**: Atomic Desegmentation Complete.
