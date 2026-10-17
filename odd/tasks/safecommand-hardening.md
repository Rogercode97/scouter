# Feature: safecommand-hardening

Locator: `odd/tasks/safecommand-hardening.md` — Engram mirror topic `odd/safecommand-hardening/tasks` (project `scouter`).
Branch: `fix/safecommand-hardening`.

## Specs

S1 — `SafeCommand` MUST reject every invocation that resolves to a shell or to a code-evaluation builtin. Today `SafeCommand(ctx, "sh", "-c", "payload")` and `SafeCommand(ctx, "eval", "payload")` both return a runnable `*exec.Cmd`; both MUST return an error. Evidence: `internal/utils/safe_exec.go` lists `"sh"` and `"bash"` in `allowedBinaries` and `"eval"`/`"source"` in `shellBuiltins`, and the argument denylist (`;|&><\``) does not catch either payload.

S2 — The three production call sites MUST keep working with no change to their arguments: `utils.SafeCommand(ctx, "rtk", "read", path, "--pointer", pointer, "--ultra-compact")` (`internal/adapters/rtk/reader.go:18`), `utils.SafeCommand(ctx, "sg", "run", "--pattern", pattern, "--rewrite", template, filePath)` (`internal/engine/structural.go:109`), `utils.SafeCommand(ctx, "git", "diff", "HEAD", "--unified=0")` (`internal/mcp/handle_intelligence.go:170`).

S3 — `CGO_ENABLED=0 go build ./...` MUST keep exiting 0.

S4 — No `#nosec` comment may assert validation the code does not implement. Both current comments claim "strict binary validation and argument sanitization in place".

S5 — The pre-existing rejections MUST NOT regress: a forbidden binary (`rm`) is rejected, and an argument containing a denylisted character is rejected.

S6 — Argument injection (leading-dash flags such as `--upload-pack=`, CWE-88) was recorded by the previous unit as a KNOWN, OPEN gap. It is **closed by T5–T7 below**; the residual limitation is stated in S7's second paragraph.

S7 — `SafeCommand` MUST reject every argument that begins with `-` unless it is exactly `--` or a flag name allow-listed for that binary (matched by name, so `--unified=3` is `--unified`). Evidence: `SafeCommand(ctx, "git", "diff", "HEAD", "--upload-pack=/bin/sh")` returned a runnable command, and none of the denylisted characters appear in that payload, so the character filter could never catch it. The three production callers MUST keep working unchanged: `rtk read <path> --pointer <p> --ultra-compact`, `<bin> run --pattern <p> --rewrite <r> <file>`, `git diff HEAD --unified=0`.

S7 limit, deliberate and recorded: `--` is accepted as a token but carries **no separator semantics**, so a positional argument that itself starts with `-` (a file literally named `-x.go`) is rejected. No caller needs that today. Granting `--` full end-of-options meaning requires tracking state across the argument list, which is the one place a subtle bug would reopen the hole; it is a separate unit if a caller ever needs it.

## Tasks

T1 — RED: table test for S1/S2/S5 plus a `FuzzSafeCommand` invariant — inline — 3402cec
T2 — Remove `"sh"`/`"bash"` from `allowedBinaries` and delete the `shellBuiltins` map and the `sh -c` branch — S1, S4 — inline — 3402cec
T3 — Verify GREEN, build, caller packages and fuzz — S2, S3 — inline — 3402cec
T4 — Record the S6 residual in this document and in the mirror — S6 — inline — 3402cec
T5 — RED: table test for S7 — seven injection cases and five legitimate caller cases; the seven fail against the unfixed wrapper and the five pass — S7 — inline — 9390dfd
T6 — Implement the per-binary flag policy in `SafeCommand` (`validateArg`, flag sets derived from the callers) — S7 — inline — 9390dfd
T7 — GREEN: `internal/utils`, build, the caller packages, fuzz, and a differential against the parent `b4047ea` held at a constant location — S2, S7 — inline — 9390dfd

## Log

L1 (user, verbatim): "se puede mejorar el proyecto digamos fixear"

L2 (user, verbatim): "ya no lo utilizare en termux solo aqui" — the Termux constraint is retired; this machine is now the only environment.

L3 (user, verbatim): "busca info web como mejoro esto"

L4 (user, verbatim): "listo el sudo pacman -S --needed go" — toolchain installed, go1.27.1.

L5 (evidence, 2026-10-17, main a1b839c): baseline `CGO_ENABLED=0 go build ./...` exit 0, `go vet ./...` exit 0, `go test ./...` exit 1 with 5 failures in `internal/engine`, all environmental (`ast-grep (sg) not found in PATH`; `sg`, `ast-grep`, `gopls` and `rtk` are not installed on this machine and no install doc declares them).

L6 (evidence): blast radius measured, not assumed — `grep -rn "SafeCommand("` returns exactly three production callers, and none passes `sh`, `bash` or a builtin. `internal/engine/executor.go` builds its own `sh -c` wrapper and does not route through `SafeCommand`, so deleting the builtin branch does not touch it.

L7 (decision): delete the whole builtin branch rather than keep the benign builtins. No caller uses any builtin, OWASP recommends not permitting `sh -c` at all, and the capability already exists where it is actually used (`executor.go`). Rejected alternative: keep benign builtins and only drop `eval`/`source`, which leaves a shell route alive with no consumer.

L8 (decision): `go test -race` cannot run under the `CGO_ENABLED=0` test mandate; verified with a minimal module — `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. Race testing, when added, runs with `CGO_ENABLED=1` on the dev machine while build and release stay CGO-free.

L9 (evidence, 3402cec): RED observed before the fix — `sh -c`, `bash -c`, `eval` and `source` returned `error = <nil>, wantErr true` while the two pre-existing rejections and the three caller cases passed; `FuzzSafeCommand` seed#2 and seed#3 showed `eval`/`source` resolving to shell `sh`. GREEN after: `internal/utils` ok, `CGO_ENABLED=0 go build ./...` exit 0, `go vet ./internal/utils/` exit 0. Fuzz 15 s: 4,872,066 executions, no counterexample. Differential against main a1b839c: baseline 5 failures in `internal/engine`, after the change the same 5 by name — no regression. `internal/mcp` and `internal/adapters/engram` ok.

L10 (evidence, S6/S7 RED observed before the fix): with the character denylist only, seven injection cases returned a runnable command — `git diff HEAD --upload-pack=/bin/sh` (the recorded S6 example), `git diff HEAD --output=/tmp/leak`, `git -c core.pager=sh`, `rtk read a.go --exec=sh`, `ast-grep run --pattern p --stdin`, `go test ./... --exec=/bin/sh`, and a flag-shaped value after an allowed flag (`rtk read a.go --pointer --upload-pack=/bin/sh`). The five legitimate caller cases passed. After the policy the suite is green.

L11 (decision): a per-binary flag allow-list, not a `--` separator. A separator only helps if every caller remembers to insert it and if the target binary honours it in the right position — two memory-dependent conditions for a security boundary. The allow-list makes the safe path the default: an unknown flag is rejected because nobody listed it, not because a caller forgot something. Rejected alternative: rejecting any `=` in an argument, which would break `--unified=0` and every legitimate attached value.

L12 (decision): the flag sets carry only flags a production caller passes — `git: --unified`, `rtk: --pointer --ultra-compact`, `sg`/`ast-grep`: `--pattern --rewrite`; `go`, `scouter`, `gopls` have none. The list is a security boundary, so it grows only together with the caller that needs the flag. A comment on the map says exactly that, because an empty `--help`-style convenience addition here is how the hole reopens.

L13 (evidence, differential with the location confound closed): the parent `b4047ea` and this unit's commit `9390dfd` were both run in a scratch worktree with `-count=1`, and both report the same five failures — `TestArchitecturalAuditIntegration`, `TestASTRuleIntegration`, `TestHealerEngine_AllFail`, `TestRippleEngine_StructuralIntegration`, `TestStructuralSearch`. An earlier pass compared the parent in a worktree against the main checkout and saw four against five; that difference was the location, not the change, which is why this pass held it constant. This branch descends from `fix/safecommand-hardening` and therefore predates the ast-grep resolution fix in #60, so the four `sg` failures are expected here and disappear once #60 lands.

L14 (evidence, GREEN): `internal/utils` ok with `-count=1`; `CGO_ENABLED=0 go build ./...` exit 0; `internal/mcp` and `internal/adapters/engram` ok.

L15 (evidence, fuzz against the new policy): `FuzzSafeCommand` ran 3,523,484 executions in 15 s with no counterexample, `PASS`. The invariant holds under generated argument vectors, not only under the table.

L16 (structure, chained): this unit is delivered as a chained PR whose base is `fix/safecommand-hardening` (PR #58) rather than `main`, because it needs that branch's hardened allow-list: cut from `main`, `sh`/`bash` and the builtins come back and the two bypasses S1 closed would reopen. The base branch tip it was built on is `b4047ea`.
