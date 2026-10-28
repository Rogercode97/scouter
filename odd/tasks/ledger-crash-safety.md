# Feature: ledger-crash-safety

Locator: `odd/tasks/ledger-crash-safety.md` — Engram mirror topic `odd/ledger-crash-safety/tasks` (project `scouter`).
Branch: `fix/ledger-crash-safety` (stacked on `docs/analysis-limits-truth-pass`, PR #72).
Parent commit: `baaa9a4`.

## Specs

S1 — The persisted Ledger MUST distinguish a staged mission from a commit that was interrupted. Today the journal on disk records only the staged patches and the file is deleted after a successful commit, so an interrupted commit is indistinguishable from a commit that never started. Evidence: `internal/engine/ledger.go` — `Ledger.Stage` calls `Save`, `CommitStaged` calls `os.Remove(l.ledgerPath)` only on success, and `Load` restores `Staged` with no notion of phase.

S2 — After a process death during the apply phase, the working tree MUST be either fully old or fully new. A half-applied tree MUST NOT be the silent outcome. Today the apply phase renames one temporary file at a time, in sorted path order, with no journal state to recover from. Evidence: `internal/engine/evolution_engine.go` — `applyPatchStep.Run` performs `os.Rename(a.path+".scouter.tmp", a.path)` per file, and `Ledger.buildCommitPlan` orders the files by path.

S3 — A successful commit MUST be durable: the applied content and the directory entry MUST survive a power loss. Today no `fsync` is issued for the temporary file before the rename, nor for the parent directory after it. Evidence: `internal/engine/evolution_engine.go` — `preparePatchStep.Run` writes the temporary file with `os.WriteFile` and never syncs it; `applyPatchStep.Run` renames and never syncs the directory.

S4 — Recovery MUST be fail-closed. When the journal shows an interrupted commit, Scouter MUST NOT continue silently: it MUST report the interrupted state, name the files known to be applied and the files still pending, and require an explicit decision to roll forward or roll back. Scouter MUST NOT guess which outcome the human wanted.

S5 — The in-process failure behaviour MUST NOT regress. A failure during the batch still rolls the whole batch back in-process, and the staged state survives for inspection afterwards. Evidence: `internal/engine/ledger_test.go` — `TestLedger_CommitFailure_PreservesLedgerStateAndRollsBack`, plus `TestLedger_CommitRollback_NonDestructive`, `TestLedger_CommitRollback_ReadOnlyFile`, `TestLedger_DeterministicStagedOrdering`, `TestLedger_BudgetEnforcement`, `TestLedger_CommitPreservesMode`.

S6 — The staging area MUST keep the original content of every touched file, because the roll-back path depends on it and `AGENTS.md` already mandates it: "Staged patches MUST populate the `Original` content from disk to prevent `Rollback` from deleting existing source files."

S7 — The command surface MUST NOT change. `scouter diff`, `scouter commit` and `scouter rollback` keep their current contract, no new mandatory flag appears, and `CGO_ENABLED=0 go build ./...` exits 0.

S8 — The interruption points MUST be exercisable deterministically. A test MUST be able to interrupt the commit at each step boundary without killing a real process and without a real power loss, so the crash-consistency property is pinned by a runnable test rather than by reasoning.

S9 — The guarantee that the code implements MUST be the guarantee that the documentation states. The doc comment in `internal/engine/ledger.go` currently claims: "applies all staged changes to the filesystem atomically using a two-phase commit" — that is false as written, because the two-phase commit is atomic only per file. `odd/specs/staging-ledger/spec.md` never mentions atomicity or interruption at all, so the spec is silent exactly where the claim is made. The README was corrected in PR #72 and states the real guarantee; the code comment and the capability spec must match it.

S10 — The negative cases MUST each fail with a specific error and MUST NOT leave a half-applied tree: a read-only target file, a missing parent directory, a target that disappears between prepare and apply, and a second commit attempt against a ledger that is already committing.

## Tasks

T1 — RED: construct the on-disk state an interrupted commit leaves (journal present, one file already replaced) and assert the ledger reports the interrupted state and refuses to continue silently, and that a roll-back restores the original content — S1, S4, S6 — inline — 93fd699
T2 — Journal phase: persist the commit phase (`staged` -> `committing` -> final) with the file list, and keep the journal until the commit is final — S1 — inline — 93fd699
T3 — Recovery: on load, detect an interrupted commit, report it with applied and pending files, and implement explicit roll-forward and roll-back, restoring `Original` content on roll-back — S4, S6, S10 — inline — 93fd699
T4 — Durability barrier: `fsync` the temporary file before the rename and the parent directory after it — S3 — inline — 66ad13e
T5 — Deterministic interruption seam so the commit can be interrupted at each step boundary, and the tests that pin the all-old-or-all-new property at every boundary — S2, S8 — inline — 66ad13e, 93fd699
T6 — GREEN: the full `internal/engine` suite, the negative cases of S10, the build, and a differential against the parent commit `baaa9a4` for the commands that share the changed code — S5, S7, S10 — inline
T7 — Documentation: correct the `ledger.go` doc comment, add the interruption and recovery scenarios to `odd/specs/staging-ledger/spec.md`, and update the Ledger section of `docs/architecture/analysis-limits.md` to the implemented guarantee — S9 — inline

## Log

L1 (user, verbatim): "¿qué evidencia necesita Scouter para afirmar que una modificación es segura y qué debe hacer cuando no puede demostrarlo?" — the question this unit answers.

L2 (user, verbatim): "Ledger crash-safety (recomendado)" — the line selected over "Precisión de impacto: FP/FN por lenguaje" and "Benchmarks comparativos con baseline".

L3 (user, verbatim): "continua" — the disposition given after the native review consent prompt was dismissed; the docs truth pass was closed and this unit opened.

L4 (evidence, verified by reading the code, not the docs): the two-phase commit in `internal/engine/apply/orchestrator.go` is atomic **per file** and rolls back **in-process** on error. It is not atomic across files, it issues no `fsync`, and it has no crash recovery. The `Ledger` persists staged patches with their original content, so the material needed to revert exists; nothing uses it after a crash.

L5 (evidence): `Ledger.Rollback` (ledger.go) only clears the in-memory state and deletes the journal file. It never writes `Original` back to disk, so calling it after an interrupted commit destroys the only baselines that could revert the tree. The in-process revert lives in `applyPatchStep.Rollback`, reachable only while the process is alive.

L6 (decision): the journal becomes a write-ahead log with an explicit phase, rather than a new command. The journal already exists, already holds the original content, and is already deleted on success — the missing piece is the phase and the recovery, so a new file format or a new command would add moving parts without changing the guarantee.

L7 (decision): the recovery is fail-closed. On finding an interrupted commit, Scouter reports the state and stops; it does not roll forward automatically, because "the intent was to apply this" is not derivable from the tree, and guessing is the failure mode this unit exists to remove.

L8 (decision): the RED test for T1 builds the crashed on-disk state directly instead of killing a process. The artifact a crash leaves is the journal plus a partially replaced tree, and that state can be constructed exactly; a real process kill would add flakiness without adding evidence. The deterministic seam in T5 exists for the boundaries inside the commit, where the state cannot be constructed by hand.

L9 (decision): `fsync` is added on the write path only (temporary file before the rename, parent directory after it). A full `syncfs` of the device is not warranted by a per-file atomicity claim, and its cost is paid on every commit.

L10 (evidence, baseline at `baaa9a4`): `CGO_ENABLED=0 go build ./...` exits 0, `CGO_ENABLED=0 go test ./...` exits 0 with every package ok, including `internal/engine` (12.5 s) and `internal/engine/apply`.

L11 (evidence, delegation failure and fallback): the unit was delegated to `gentle-ai-worker` twice, as the context backstop requires. The first run failed after 43 turns and left 685 changed lines; the second failed again. The tree was not salvageable: `internal/engine/ledger.go` contained the worker's own planning prose and a mangled tool invocation inside the source, so the file did not parse. The changes were reverted to `baaa9a4` and the unit was executed inline, with the scope reduced to two reviewable commits. The delegation mechanism did not respond, so it was not retried again.

L12 (evidence, RED observed before the fix): the rollback did not restore the file the interrupted commit had already replaced.

```
--- FAIL: TestLedger_RollbackRestoresAfterInterruptedCommit (0.00s)
    ledger_crash_test.go:55: target content = "updated content\n", want the original "original content\n"
```

L13 (decision, added while implementing): a pending file whose content is exactly the staged content is treated as applied. The journal is written after each replacement, so a crash between the replacement and that write leaves a file the journal still calls pending; without this rule the recovery would leave the tree half applied while claiming to have resolved it.

L14 (decision, added while implementing): a rollback refuses to overwrite a file whose content changed after the interruption, and fails instead. The journal cannot know about work that happened outside it, and destroying that work to complete a rollback would be worse than refusing.

L15 (decision, scope boundary): the roll-forward is exposed on the engine API and not on the CLI. S7 forbids changing the command surface, so `scouter rollback` is the only resolution reachable from the command line for now; the limits document states that the roll-forward is not yet exposed there.

L16 (evidence, GREEN): `CGO_ENABLED=0 go test ./internal/engine/ -run TestLedger -count=1` reports every pre-existing ledger test passing (budget, staging, mode preservation, commit failure rollback, baseline capture, deterministic ordering) plus the six new ones. `CGO_ENABLED=0 go test ./... -count=1` exits 0 with every package ok, `internal/engine` in 13.1 s. `CGO_ENABLED=0 go build ./...` exits 0 and `go vet ./internal/engine/...` is clean.

L17 (evidence, differential against `baaa9a4`): the binaries of the parent commit and of this branch were built in a `mktemp -d` scratch checkout and run against the same working directory. `scouter diff`, `scouter commit` and `scouter rollback` produce identical stdout, stderr and exit codes on both.

```
### diff        No changes staged in Ledger.      exit=0
### commit     No changes staged in Ledger.      exit=0
### rollback   ✅ Ledger rolled back. All staged changes cleared.   exit=0
```

L18 (evidence, slice boundary): the unit landed as two commits, `66ad13e` (the interruption seam and the durability barrier, 142 lines) and `93fd699` (the journal phase, the recovery and their tests, 630 lines). The second exceeds the ~400-line review slice because the recovery and the tests that pin it are one behaviour: splitting them would put the tests in a commit whose behaviour they do not describe. The documentation landed separately with this record.
