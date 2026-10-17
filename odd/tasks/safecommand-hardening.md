# Feature: safecommand-hardening

Locator: `odd/tasks/safecommand-hardening.md` — Engram mirror topic `odd/safecommand-hardening/tasks` (project `scouter`).
Branch: `fix/safecommand-hardening`.

## Specs

S1 — `SafeCommand` MUST reject every invocation that resolves to a shell or to a code-evaluation builtin. Today `SafeCommand(ctx, "sh", "-c", "payload")` and `SafeCommand(ctx, "eval", "payload")` both return a runnable `*exec.Cmd`; both MUST return an error. Evidence: `internal/utils/safe_exec.go` lists `"sh"` and `"bash"` in `allowedBinaries` and `"eval"`/`"source"` in `shellBuiltins`, and the argument denylist (`;|&><\``) does not catch either payload.

S2 — The three production call sites MUST keep working with no change to their arguments: `utils.SafeCommand(ctx, "rtk", "read", path, "--pointer", pointer, "--ultra-compact")` (`internal/adapters/rtk/reader.go:18`), `utils.SafeCommand(ctx, "sg", "run", "--pattern", pattern, "--rewrite", template, filePath)` (`internal/engine/structural.go:109`), `utils.SafeCommand(ctx, "git", "diff", "HEAD", "--unified=0")` (`internal/mcp/handle_intelligence.go:170`).

S3 — `CGO_ENABLED=0 go build ./...` MUST keep exiting 0.

S4 — No `#nosec` comment may assert validation the code does not implement. Both current comments claim "strict binary validation and argument sanitization in place".

S5 — The pre-existing rejections MUST NOT regress: a forbidden binary (`rm`) is rejected, and an argument containing a denylisted character is rejected.

S6 — Argument injection (leading-dash flags such as `--upload-pack=`, CWE-88) is a KNOWN, OPEN gap that this unit does NOT close. It stays recorded here; no comment or test may imply it is fixed.

## Tasks

T1 — RED: table test for S1/S2/S5 plus a `FuzzSafeCommand` invariant — inline — 4402e15
T2 — Remove `"sh"`/`"bash"` from `allowedBinaries` and delete the `shellBuiltins` map and the `sh -c` branch — S1, S4 — inline — 4402e15
T3 — Verify GREEN, build, caller packages and fuzz — S2, S3 — inline — 4402e15
T4 — Record the S6 residual in this document and in the mirror — S6 — inline — 4402e15

## Log

L1 (user, verbatim): "se puede mejorar el proyecto digamos fixear"

L2 (user, verbatim): "ya no lo utilizare en termux solo aqui" — the Termux constraint is retired; this machine is now the only environment.

L3 (user, verbatim): "busca info web como mejoro esto"

L4 (user, verbatim): "listo el sudo pacman -S --needed go" — toolchain installed, go1.27.1.

L5 (evidence, 2026-10-17, main a1b839c): baseline `CGO_ENABLED=0 go build ./...` exit 0, `go vet ./...` exit 0, `go test ./...` exit 1 with 5 failures in `internal/engine`, all environmental (`ast-grep (sg) not found in PATH`; `sg`, `ast-grep`, `gopls` and `rtk` are not installed on this machine and no install doc declares them).

L6 (evidence): blast radius measured, not assumed — `grep -rn "SafeCommand("` returns exactly three production callers, and none passes `sh`, `bash` or a builtin. `internal/engine/executor.go` builds its own `sh -c` wrapper and does not route through `SafeCommand`, so deleting the builtin branch does not touch it.

L7 (decision): delete the whole builtin branch rather than keep the benign builtins. No caller uses any builtin, OWASP recommends not permitting `sh -c` at all, and the capability already exists where it is actually used (`executor.go`). Rejected alternative: keep benign builtins and only drop `eval`/`source`, which leaves a shell route alive with no consumer.

L8 (decision): `go test -race` cannot run under the `CGO_ENABLED=0` test mandate; verified with a minimal module — `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. Race testing, when added, runs with `CGO_ENABLED=1` on the dev machine while build and release stay CGO-free.

L9 (evidence, 4402e15): RED observed before the fix — `sh -c`, `bash -c`, `eval` and `source` returned `error = <nil>, wantErr true` while the two pre-existing rejections and the three caller cases passed; `FuzzSafeCommand` seed#2 and seed#3 showed `eval`/`source` resolving to shell `sh`. GREEN after: `internal/utils` ok, `CGO_ENABLED=0 go build ./...` exit 0, `go vet ./internal/utils/` exit 0. Fuzz 15 s: 4,872,066 executions, no counterexample. Differential against main a1b839c: baseline 5 failures in `internal/engine`, after the change the same 5 by name — no regression. `internal/mcp` and `internal/adapters/engram` ok.
