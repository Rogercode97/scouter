# ✅ TASKS: Divine Linker (Wave 11)

## Phase 1: Stability (LSP Lifecycle)
- [ ] `internal/engine/lsp/manager.go`: Ensure `Close()` is robust and idempotent.
- [ ] `internal/mcp/server.go`: Add `func (s *Server) Close() error` and invoke `s.lspMgr.Close()`.
- [ ] `internal/cli/cli.go`: Update the `mcp` command to handle context cancelation and call `server.Close()`.
- [ ] `cmd/scouter/main.go`: Ensure signals (SIGINT, SIGTERM) trigger context cancelation in `cli.Run`.

## Phase 2: Decoupling & Store
- [ ] `internal/store/store.go`: 
    - [ ] Add `GetInterfaces(ctx) ([]Symbol, error)` to fetch all interface symbols.
    - [ ] Rename/Refactor `SaveCall` to be more generic if needed or add `SaveLink` for implementations.
    - [ ] Mark `ResolveInterfaces` as deprecated or delete it.

## Phase 3: The Linker Engine
- [ ] `internal/engine/linker.go`: Implement `LinkInterfaces(ctx, repo, lspMgr)`.
    - [ ] Loop through symbols from `repo.GetInterfaces`.
    - [ ] For each, call `lspMgr.Implementation`.
    - [ ] Map `lsp.Location` back to `store.Call` with `LinkType: "implements"`.
    - [ ] Wrap everything in `repo.WithTransaction`.

## Phase 4: Integration & Verification
- [ ] `internal/mcp/handlers.go`: Update `handleIndex` to call `engine.LinkInterfaces` instead of `store.ResolveInterfaces`.
- [ ] Manual Verification: Index a file with interfaces and verify `implements` links via `scouter_callers`.
- [ ] `just build`: Ensure project integrity.
