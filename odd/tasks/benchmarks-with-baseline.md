# Feature: benchmarks-with-baseline

Locator: `odd/tasks/benchmarks-with-baseline.md` — Engram mirror topic `odd/benchmarks-with-baseline/tasks` (project `scouter`).
Branch: `perf/benchmarks-with-baseline` (from `main`).
Parent commit: `e06750c` (main at the time of writing).

## Specs

S1 — A developer MUST be able to run the benchmark procedure with one documented command. Today there is no benchmark target: `justfile` exposes `run`, `build`, `test`, `fmt` and `sync`, and the `Makefile` exposes `all`, `build`, `clean`, `run`, `test` — none of them runs a benchmark. Evidence: `grep -n "bench" justfile Makefile` returns nothing.

S2 — The comparison MUST have a conventional baseline. The same task MUST be run through Scouter and through a conventional text search (`ripgrep`, installed at `/usr/bin/rg`, version 15.2.0 on the reference machine), over the same repository, so the two numbers are comparable. A comparison with no baseline measures nothing.

S3 — The comparison MUST report the token cost, not only the latency. The claim Scouter makes to an agent is that it answers with less text for the same question; a comparison that reports only milliseconds does not test that claim. The token count MUST use the same approximation the engine itself uses, so the numbers are comparable with the engine's own accounting: `len(content)/4` (`internal/engine/ledger.go`, `kiIncurred := int64(len(patch.NewContent) / 4)`).

S4 — The comparison MUST report the index cost: the time to index the corpus and the size of the resulting index. A structural engine that needs a long index to answer one question has to be measured against that cost, not hidden by it.

S5 — The results MUST be recorded in the repository, together with the environment they were measured in: the commit, the Go version, the CPU count, the corpus size and the file count. Without the environment a number is not a baseline; with it, a later run can be compared against it.

S6 — The comparison MUST state what it does not measure. `ripgrep` does not understand symbols, so it is not a competitor for the same task: it finds text, Scouter resolves structure. The claim has to be bounded to what was actually measured, or it is a claim about nothing.

S7 — Zero CGO, no new module dependency, and the benchmark MUST NOT become a CI gate. A timing assertion is a flaky gate, and a flaky gate gets disabled. The CI story is a smoke test with a hard time budget, or an explicit exclusion with the reason recorded.

S8 — The harness MUST NOT add `rg`, or any new binary, to the SafeCommand allow-list. `internal/utils/safe_exec.go` states the rule: "Every flag listed here exists because a production caller passes it; add one only together with the caller that needs it." A benchmark harness is not a production caller, so the comparison belongs in a developer-run script, outside the product's execution path.

S9 — The existing benchmarks MUST keep working. `BenchmarkMassiveIndexing` (`internal/engine/benchmark_test.go`), `BenchmarkStructuralSearch_Sequential` and `BenchmarkStructuralSearch_Concurrent` (`internal/engine/concurrency_test.go`), `BenchmarkNewStoreOpen` and `BenchmarkNewStoreOpenOLDDatabase` (`internal/store/migrate_test.go`) are the current inventory; none of them may regress or be silently removed.

## Tasks

T1 — Inventory and baseline record: run the five existing benchmarks on the reference machine and record the numbers, the environment and the commit — S1, S5, S9 — inline — recorded below
T2 — Add `scripts/bench-baseline.sh`: the same task through Scouter and through ripgrep over the same repository snapshot, reporting latency, output bytes and tokens, and the index cost (time to index plus index size) — S2, S3, S4, S8 — worker — delivered
T3 — Add the `just bench` recipe with a fixed, documented procedure, and the machine profile it prints — S1 — worker — delivered
T4 — Run the procedure and write `docs/benchmarks/results.md` with the environment, the exact commands, the numbers and an explicit statement of what the comparison does not measure — S5, S6 — inline
T5 — Verify: the script runs end to end from a clean checkout, `CGO_ENABLED=0 go build ./...` exits 0, `CGO_ENABLED=0 go test ./...` stays green and fast, and the existing benchmarks still run — S7, S9 — inline

## Log

L1 (user, verbatim): "Benchmarks comparativos con baseline" — the line selected after the ledger crash-safety unit, over "Precisión de impacto: FP/FN por lenguaje".

L2 (source, verbatim, from the external audit of the public repository): "Comparar Scouter con búsquedas convencionales y otras herramientas de análisis estructural en repositorios reales. Medir latencia, memoria, precisión, consumo de tokens y coste de indexación." — this unit covers the latency, token and indexing parts of that sentence. The precision part belongs to the impact-accuracy line, which is a different unit.

L3 (evidence, verified): the repository has no benchmark target. `justfile` and `Makefile` expose no `bench` recipe, and the five existing `func Benchmark` live in `internal/engine/benchmark_test.go`, `internal/engine/concurrency_test.go` and `internal/store/migrate_test.go`. CI never runs them: `.github/workflows/ci.yml` runs `CGO_ENABLED=0 go test -race -v -p 1 ./...`, with no `-bench`.

L4 (decision): the comparison is a developer-run shell script, not a Go test that shells out. `rg` is not in the SafeCommand allow-list, and adding it would widen a security boundary for a non-production caller, which `internal/utils/safe_exec.go` explicitly forbids. A script the developer invokes is outside the product's execution path and needs no allow-list entry.

L5 (decision): tokens are measured as `bytes/4`, the same approximation the engine uses for its own budget accounting, so the comparison and the engine do not disagree about what a KiB of content is.

L6 (decision): no timing gate in CI. The benchmark records numbers; it does not assert them. A gate would be a flaky gate, and the repository's own CI already runs the race detector, which is slow enough.

L7 (evidence, reference machine): Go 1.27.1-X:nodwarf5 linux/amd64, 12 CPUs, ripgrep 15.2.0 at `/usr/bin/rg`. The corpus is the repository itself, because it is the corpus the tool claims to be fast on.

L8 (decision): the branch starts from `main`, not from the ledger branch. The unit touches `scripts/`, `justfile`, `docs/benchmarks/` and the benchmark test files; it does not depend on the ledger crash-safety work or on the documentation truth pass, and keeping it independent keeps its diff reviewable on its own.

L9 (evidence, T1 baseline record, commit `e06750c`, `CGO_ENABLED=0 go test -bench . -benchtime=1x -count=1`): environment Go 1.27.1-X:nodwarf5 linux/amd64, CPU AMD Ryzen 5 5600GT, 12 procs, ripgrep 15.2.0 at `/usr/bin/rg`.

| Benchmark | ns/op | what it actually measures |
| --- | --- | --- |
| `BenchmarkMassiveIndexing` | 17,879,043,194 (17.88 s) | indexes 1000 synthetic Go files of four lines each, into a fresh SQLite store: about 17.9 ms per file |
| `BenchmarkStructuralSearch_Sequential` | 6,262,353 (6.26 ms) | `StructuralSearch` over 50 synthetic files with the pattern `println($X)` |
| `BenchmarkStructuralSearch_Concurrent` | 3,005,009 (3.01 ms) | the same call on the same corpus |
| `BenchmarkNewStoreOpen` | 651,593 (0.65 ms) | opening a fresh store |
| `BenchmarkNewStoreOpenOLDDatabase` | 639,861 (0.64 ms) | opening a store whose database predates the current schema |

L10 (discovery, from reading the benchmark sources): `BenchmarkStructuralSearch_Sequential` and `BenchmarkStructuralSearch_Concurrent` call the same function on the same corpus, and the source says so: "Since StructuralSearch is now concurrent, this name is just for comparison if we wanted to revert to sequential." The 2.08x gap between them is warm-cache noise from the second run, not a sequential-versus-concurrent measurement. The benchmark names promise a comparison the code no longer provides, and any results document that quotes them as such would be quoting a fiction. The pair is left untouched by this unit (S9) and the results document states what it measures instead.

L11 (observation, from the same record): the index cost dominates the query cost by three orders of magnitude on this corpus, about 17.9 ms per indexed file against about 6 ms for a structural search over 50 files. A comparison that reports only query latency would hide the number that decides whether the tool is worth its setup cost on a real repository, which is why S4 asks for the index cost explicitly.

L12 (evidence, T2/T3 delivered and independently verified): the worker wrote `scripts/bench-baseline.sh` and the `just bench` recipe. Verified by inspection and by running it: `set -euo pipefail`, every external binary resolved with `command -v` and invoked through a quoted variable, no `eval`, no `sh -c`, the temporary directory and the isolated database removed by an `EXIT` trap, and `internal/utils/safe_exec.go` untouched (S8 held). The isolation mechanism is real: `SCOUTER_CONFIG` is honoured by `internal/config/config.go:112`, so the measured index is the one the script created. With `rg` removed from `PATH` the script still exits 0 and reports the conventional baseline as unavailable.

L13 (evidence, the comparison, three runs): index 12,488 / 12,630 / 12,673 ms, index size 14,901,248 / 14,872,576 / 15,060,992 bytes; Scouter 47 / 48 / 48 ms and 863 bytes (215 tokens) in every run; ripgrep 5 / 6 / 6 ms and 2,572 / 2,572 / 3,946 bytes (643 / 643 / 986 tokens). The corpus is 252 indexed files and 426 git-tracked files.

L14 (discovery, the most useful of the three runs): the third run measured the tree after this document and the script were added, and the conventional baseline's answer grew by 1,374 bytes while Scouter's did not change by one byte. The text baseline matches the query string wherever it appears, including in prose that merely names it, while the structural answer is the same eight symbols either way. This is a genuine property of the two tools and also a fairness hazard, because the documentation of the comparison inflates the baseline. The results document therefore quotes the least favourable figures for Scouter (runs A and B) and discloses run C.

L15 (decision): the headline claim is bounded to what was measured. Scouter is not faster — 48 ms against 6 ms — and the index costs 12.6 s and 14.9 MB for 252 files, which is the number that decides the trade on a repository where an agent asks one question. The claim that survives is that the same question costs an agent a third of the text through Scouter, and that the text is structural.

L16 (decision): precision and recall are not claimed here. `ripgrep` matches text and Scouter resolves symbols, so they are not answering the same question and scoring them against each other would be meaningless. The impact-accuracy unit owns that measurement and it is not published yet.

L17 (evidence, T5 verification): `just bench` exits 0 and prints the profile and the metrics; `CGO_ENABLED=0 go build ./...` exits 0; `CGO_ENABLED=0 go test ./... -count=1` exits 0 with every package ok; the five existing benchmarks are untouched and still produce the numbers recorded in L9.
