# Tasks: Ripple V2 Deepening (Cross-Package Type Resolution)

## Phase 1: Foundation (Store & Types)
- [x] 1.1 Update `Symbol` struct in `internal/types/types.go`
- [x] 1.2 Update `Symbol` struct in `internal/store/store.go` and add columns in `migrate()`
- [x] 1.3 Update `SaveSymbol` and `GetSymbols` in `internal/store/store.go`
- [x] 1.4 Test: Verify SQLite migration

## Phase 2: Semantic Parser (Extraction)
- [x] 2.1 Refactor `internal/engine/parser.go` to use `go/packages`
- [x] 2.2 Implement `PackagePath` extraction
- [x] 2.3 Implement `ReceiverType` detection
- [x] 2.4 Test: Verify extraction logic

## Phase 3: Ripple Engine V2 (Analysis)
- [x] 3.1 Update `AnalysisEngine` to load type universe
- [x] 3.2 Implement `ResolveInterfaces` using `go/types.Implements`
- [x] 3.3 Ensure method set rules are applied
- [x] 3.4 Save `implements` and `satisfies` links

## Phase 4: Verification & Integration
- [x] 4.1 Create cross-package integration test
- [x] 4.2 Scenario Test: Verify interface satisfaction across packages
- [x] 4.3 Scenario Test: Verify pointer receiver validation
- [x] 4.4 Scenario Test: Verify embedded interface resolution

## Phase 5: Cleanup & Docs
- [x] 5.1 Update CLI output or logs
- [x] 5.2 Update `docs/ARCHITECTURE.md`
