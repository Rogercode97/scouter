# Benchmark Results

The numbers Scouter can defend, measured against a conventional text search over the same repository snapshot, with the procedure and the environment that produced them.

## How to reproduce

```bash
just bench
```

or directly:

```bash
./scripts/bench-baseline.sh
```

The procedure is fixed and self-describing: it builds the binary from the current checkout into a temporary directory, indexes the repository with `scouter index .` while timing it and measuring the resulting index, then runs the same query through `scouter search` and through `rg -n --`, timing each and measuring its output. The script prints the machine profile it measured on. It needs no network, and when `ripgrep` is not on `PATH` it reports the conventional baseline as unavailable and still reports the Scouter side. A ripgrep that is present but fails aborts the run with its error instead of being recorded as an empty baseline: a broken tool must not read as a measurement.

The query is `IndexerPipeline`, chosen because it has a meaningful answer in this repository and because both tools can be asked it identically.

## Environment

| | |
| --- | --- |
| Base commit | `e06750cb914af371a33f8db250569fe35b6cc122` (`main`) — the tree the harness was written against |
| Harness commit | `0138677` — the commit that carries `just bench` and `scripts/bench-baseline.sh` |
| Go | `go1.27.1-X:nodwarf5 linux/amd64` |
| CPU | AMD Ryzen 5 5600GT, 12 procs |
| ripgrep | 15.2.0 at `/usr/bin/rg` |
| Corpus | this repository; 252 files indexed, 426 files tracked by git |

## The comparison

Three runs of the same procedure, to show which numbers move and which do not:

| Measurement | Run A | Run B | Run C |
| --- | --- | --- | --- |
| Index time | 12,488 ms | 12,630 ms | 12,673 ms |
| Index size | 14,901,248 B | 14,872,576 B | 15,060,992 B |
| Scouter latency | 47 ms | 48 ms | 48 ms |
| Scouter output | 863 B (215 tokens) | 863 B (215 tokens) | 863 B (215 tokens) |
| ripgrep latency | 5 ms | 6 ms | 6 ms |
| ripgrep output | 2,572 B (643 tokens) | 2,572 B (643 tokens) | 3,946 B (986 tokens) |

Runs A and B measured the same tree. Run C measured the tree **after** this document and the benchmark script were added to it, and it is the most instructive of the three: the conventional baseline's answer grew by 1,374 bytes while Scouter's did not change at all, by a single byte.

The reason is visible in the output. The text baseline matches the literal string everywhere it appears, including in prose that merely names the query — this document names it, the script contains it as a variable, and both are now part of the corpus. Scouter resolves the query to symbols, so the set of answers is the same eight symbols whether or not the repository talks about `IndexerPipeline` in a sentence.

That is a real property of the two tools, and it is also the reason the comparison is reported with the **least favourable** figures for Scouter: the headline below uses run A and run B, where the baseline's answer is smallest. Run C is disclosed because hiding it would make the claim look better than it is.

No single commit reproduces all three columns. All three runs were taken from working trees while the harness was being written, which is why the environment table records a base commit and a harness commit instead of claiming one SHA for the whole table. What a reader reproduces by checking out a commit that contains this document and running `just bench` is the procedure and Scouter's side of the table: 863 B in every run. The baseline's figure depends on which files the tree holds, and that dependence is exactly what run C measures.

Tokens are bytes/4, the same approximation `internal/engine/ledger.go` uses for its own budget accounting (`kiIncurred := int64(len(patch.NewContent) / 4)`), so these numbers and the engine's own accounting do not disagree about what a KiB of content is.

## What the numbers say

**Scouter is slower and its answer is smaller.** The query costs 48 ms through Scouter against 6 ms through ripgrep, and it returns 863 bytes against 2,572 — about a third of the text, or 215 tokens against 643. The two answers are not the same kind of thing:

```
🔍 Search results for "IndexerPipeline":
- [class] IndexerPipeline (/home/roger/Projects/scouter/internal/engine/indexer.go:51)
- [function] NewIndexerPipeline (/home/roger/Projects/scouter/internal/engine/indexer.go:55)
- [method] IndexerPipeline.Index (/home/roger/Projects/scouter/internal/engine/indexer.go:179)
- [method] IndexerPipeline.indexDirectory (/home/roger/Projects/scouter/internal/engine/indexer.go:257)
- [function] IndexerPipeline.indexDirectory.func1 (/home/roger/Projects/scouter/internal/engine/indexer.go:288)
...
```

against ripgrep's text locations:

```
./tests/semantic_ripple_test.go:53:	indexer := engine.NewIndexerPipeline(engine.IndexerConfig{Store: db, Analyzer: analyzer})
./tests/interface_propagation_test.go:57:	indexer := engine.NewIndexerPipeline(...)
./scripts/bench-baseline.sh:14:QUERY="IndexerPipeline"
./cmd/scouter/scoutercmd/mcp.go:65:			Indexer: engine.NewIndexerPipeline(...)
```

Scouter answers with the symbol and its members; ripgrep answers with every line that happens to contain the string. The claim that survives measurement is therefore not "Scouter is faster" — it is not — but "for the same question, Scouter hands an agent a third of the text, and that text is structural".

**The index cost dominates, and it is the number that decides the trade.** Building the index costs 12.6 s and 14.9 MB for 252 files. A single query against that index then costs 48 ms. Any comparison that reports only the query hides that ratio; on a repository where an agent asks one question, 12.6 s of indexing is not amortized by anything.

**The output size is deterministic; the latency is not.** The byte counts are identical across runs A and B, because they depend on the corpus and not on the machine. The latencies move by a millisecond or two, and the index time and size by about one percent. Treat the millisecond figures as noise and the byte figures as facts. The one byte figure that did change is the baseline's, in run C, and it changed because the corpus changed, not because the measurement drifted.

## What this comparison does not measure

- **Precision or recall.** Nothing here says whether Scouter found every call site or every definition. `ripgrep` matches text and Scouter resolves symbols, so they are not answering the same question; scoring them against each other would be meaningless. Precision belongs to the impact-accuracy measurement, which is a separate unit and is not published yet.
- **Memory.** Neither side is measured for resident memory.
- **The cost of the answer being right.** A smaller answer is only better if it is complete. The byte counts say nothing about that.
- **Anything about a non-Go corpus.** The corpus here is a Go repository, which is the language Scouter parses natively. The tree-sitter path for other languages is not measured at all.
- **The sequential-versus-concurrent claim.** `BenchmarkStructuralSearch_Sequential` and `BenchmarkStructuralSearch_Concurrent` call the same function on the same corpus — the source says so: "Since StructuralSearch is now concurrent, this name is just for comparison if we wanted to revert to sequential." The 6.26 ms against 3.01 ms between them is warm-cache noise, not a concurrency measurement. Both benchmarks are left untouched, and neither number is quoted here as a comparison.

## One fairness note

The corpus contains the benchmark harness itself, so the conventional baseline matches the literal query string inside `scripts/bench-baseline.sh` (line 14). After this document landed, the corpus also contains prose that names the query six times. Every one of those is a text match for the baseline and none of them is a symbol, so the baseline's byte count carries them and Scouter's does not. It is left in place rather than special-cased, because the alternative is a corpus that exists only to flatter one side. The effect is visible as the difference between run B (2,572 B) and run C (3,946 B) in the baseline column, and the headline figures quoted above are the ones from before this document existed.

The two tools also read different file sets, and the difference does not run in a single direction. The indexer keeps only its supported extensions, skips the blocked directories, and skips files above 64 KiB; it does not read `.gitignore` at all. The baseline honours `.gitignore` and applies no extension filter. Each side therefore sees files the other does not: this document, the shell script and anything outside the supported extensions are text matches only the baseline can make, while a supported file inside an ignored directory is something only Scouter can index. Neither side is corrected for that, because correcting it would mean teaching the baseline Scouter's indexing rules and turning a comparison of two tools into a comparison of two configurations of the same filter.

The byte gap reported above is an observed difference between two different filters, not a bound on the difference in output size. It can move in either direction depending on which files a tree holds. What the measurement supports is a comparison of the answers each tool gives under its own rules, not a claim about how they would compare over an identical file set.

## The existing benchmark inventory

The five `func Benchmark` in the repository, measured separately with `CGO_ENABLED=0 go test -bench . -benchtime=1x -count=1` on the same machine:

| Benchmark | ns/op | What it measures |
| --- | --- | --- |
| `BenchmarkMassiveIndexing` | 17,879,043,194 (17.88 s) | indexes 1000 synthetic four-line Go files into a fresh store, about 17.9 ms per file |
| `BenchmarkStructuralSearch_Sequential` | 6,262,353 (6.26 ms) | `StructuralSearch` over 50 synthetic files, pattern `println($X)` — same code path as the next row |
| `BenchmarkStructuralSearch_Concurrent` | 3,005,009 (3.01 ms) | the same call on the same corpus |
| `BenchmarkNewStoreOpen` | 651,593 (0.65 ms) | opening a fresh store |
| `BenchmarkNewStoreOpenOLDDatabase` | 639,861 (0.64 ms) | opening a store whose database predates the current schema |

None of them runs in CI: `.github/workflows/ci.yml` runs `CGO_ENABLED=0 go test -race -v -p 1 ./...` with no `-bench`. The numbers above are recorded here so a later change can be compared against them; no timing assertion is enforced anywhere, because a timing gate is a flaky gate.
