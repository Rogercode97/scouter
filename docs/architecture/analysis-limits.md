# Analysis Limits and Guarantees

This document states what each Scouter analysis surface can and cannot see, so an agent can decide when to trust a result and when to escalate to a human. Every claim below points at the code that produces it.

A limit is not a defect report. It is the boundary of a guarantee, written down so that no consumer has to infer it.

## How to read this document

- **Guarantee** — behavior the code enforces, and therefore a test can pin.
- **Limit** — behavior the code does not resolve. A clean result under a limit is not evidence of safety.
- **Advisory** — the tool informs a decision but does not enforce it.

The analysis layers are static. They describe the code that is written, not the program that runs.

---

## Parsing and indexing

**Guarantee.** Go sources are parsed with the standard library AST (`internal/engine/parser.go`), which yields the full symbol, call and data-flow set, and supports SSA analysis for variable-level blast radius (`internal/engine/impact.go`).

**Guarantee.** Other languages are parsed with Tree-sitter queries registered in `internal/engine/treesitter.go`: `.ts`, `.tsx`, `.jsx`, `.js`, `.py` and `.rs`. Symbols, calls and imports are extracted for each; a data-flow query exists for the TypeScript/JavaScript, Python and Rust configurations.

**Limit.** Non-Go languages are resolved through Tree-sitter queries, not a type checker. Their call graph is shallower than Go's: generics, mixins, decorators, overloads and re-exports are approximated by the query patterns, and an unmodelled construct is silently absent from the graph rather than reported as unresolved.

**Limit.** Files larger than 5 MiB are rejected (`MaxParseSize` in `internal/engine/parser.go`) and are not indexed.

## Definition and type information

**Guarantee.** The LSP-backed surfaces — `ast_definition`, `ast_type_info` and call-hierarchy resolution — delegate to a language server managed in `internal/engine/lsp/manager.go`.

**Limit.** Only `gopls` is configured. For any other extension the manager returns `no LSP server configured for extension <ext>`, so definition and type information are Go-only. A missing `gopls` binary is an error, not a degraded result: the tool reports the failure instead of guessing.

## Structural search

**Guarantee.** `ast_structural_search` delegates to the external `ast-grep` binary through the filter registry (`internal/adapters/astgrep/searcher.go`, `internal/filter/astgrep.go`), with an explicit language for `.go`, `.ts`, `.js`, `.rs` and `.py`.

**Limit.** `ast-grep` is not bundled. Without it on `PATH` the search fails with `ast-grep binary not found (tried 'sg' and 'ast-grep')`. It is a subprocess, so it is subject to that tool's own parsing limits.

## Impact analysis and ripple

**Guarantee.** `impact` and `ledger_ripple` traverse the indexed call graph with a bounded depth: default 3, hard maximum 10 (`internal/engine/impact.go`).

**Guarantee.** Interface implementations are modeled statically (`internal/engine/interfaces.go`), so a call through an interface is expanded to the implementations that the graph knows.

**Limit.** Static dispatch only. Runtime dispatch that the graph cannot express is not resolved: interface values chosen at runtime, reflection, function values stored in structs or maps, plugin and registry lookups, dependency-injection containers, generated code, and code invoked from strings. A result that lists one caller does not mean there is one caller.

**Limit.** The graph only contains what was indexed. A repository that was not indexed with `scouter index` produces an empty answer, not an error.

## Test prediction and churn risk

**Guarantee.** `risk_predict` maps the symbols touched by the current git diff to test files through the call relation recorded at index time (`internal/engine/impact.go`).

**Limit.** The relation is a static graph edge. A test that exercises the change through a path the graph does not contain — an interface the test implements, a mocked dependency, an external process — is not predicted. The result is a list of candidate tests, not a coverage proof.

**Limit.** Churn scoring (`critical`, `risk_critical_code`) is derived from the depth of the local git history. A shallow clone or a fresh fork changes the numbers, and the same commit scores differently in two clones.

## Semantic search

**Guarantee.** Search uses the embedding provider when one is configured and falls back to SQLite FTS5 text matching when it is not (`internal/engine/search.go`).

**Limit.** Without a provider, semantic ranking is unavailable and the result quality is lexical. `sqlite-vec` is an optional per-database extension; a database without it answers text queries only.

## The Staging Ledger

This is the component that mutates a codebase, so its contract is stated precisely.

**Guarantee.** Staging persists every patch together with the original content of the file to `.scouter/staging/ledger.json` (`internal/engine/ledger.go`). The original content is what makes a rollback possible.

**Guarantee.** A commit is two-phase (`internal/engine/apply/orchestrator.go`): the prepare phase writes `<file>.scouter.tmp` for every staged file, and the apply phase renames each temporary file over its target. A rename is atomic, so **each file is replaced atomically**.

**Guarantee.** A failure during the batch rolls the whole batch back in-process, and the staged state survives for inspection afterwards (`internal/engine/ledger.go`, `TestLedger_CommitFailure_PreservesLedgerStateAndRollsBack`).

**Limit.** **The batch is not atomic across files.** Files are applied one at a time, in sorted path order. An interruption between two renames leaves the tree half-applied: some files new, some old.

**Limit.** **There is no crash recovery.** The persisted ledger records what was staged, not what was applied, so an interrupted commit is indistinguishable from a commit that was never started. Nothing replays or reverts it automatically.

**Limit.** **Durability under power loss is not guaranteed.** No `fsync` is issued for the temporary file or for the directory entry after the rename.

**Limit.** `scouter rollback` and `ledger_rollback` clear the staging area. They do not revert files that were already written to disk. Reverting an applied change is what the in-process rollback does, and it is not reachable from the command line after the fact.

**Advisory.** Impact analysis is not enforced by the commit path. `scouter commit` applies what was staged; running `scouter impact` first is a discipline, not a gate.

**Operational consequence.** Read `scouter diff` before `scouter commit`. After an interrupted commit, inspect the working tree with `git status` and reconcile by hand before staging anything else.

## What Scouter cannot answer

- Whether a change is semantically correct.
- Whether the existing tests actually cover the change.
- Whether a runtime path reaches code that static analysis cannot see.
- Anything about a repository that has not been indexed.

## Known gaps

- **Ledger crash consistency.** The ledger is not a write-ahead log: it has no `committing` state, no roll-forward, no roll-back after an interruption, and no durability barrier. The planned work is a journal state machine with a fail-closed recovery path, exercised by fault injection at every step boundary.
- **No published accuracy numbers.** The fixtures under `tests/fixtures/` exercise ripple and interface propagation, but precision and recall per language are not measured or published yet.

## How these claims stay true

The surface described in the README is pinned by tests, so the documentation cannot drift away from the binary:

- `cmd/scouter/scoutercmd/readme_docs_test.go` proves that every `scouter <command>` the README shows is a registered command.
- `internal/mcp/readme_docs_test.go` proves that the MCP tool and prompt inventory in the README matches the server registrations, in both directions, and that the stated tool count is the real one.

Both run in CI. A change that adds, removes or renames a surface must update the README in the same commit.
