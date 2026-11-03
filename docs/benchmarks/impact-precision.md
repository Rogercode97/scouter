# Impact Precision

Whether `scouter impact` finds the callers that exist, measured against an answer key read from the source. This document owns the precision and recall numbers; `docs/benchmarks/results.md` owns the speed numbers.

## Why this exists

Speed was the question that surfaced it: on this machine `scouter index .` takes 12.5 s for 265 files and `scouter search` answers in 48 ms against ripgrep's 5 ms. Optimizing that would have made a specific answer fast, and nothing said the answer was right. So the order was inverted: measure correctness first, at the layer where `impact` decides what to print.

## How to reproduce

```bash
# Python, end to end through the command. Runs in the normal suite.
CGO_ENABLED=0 go test ./tests/ -run TestImpactPrecision_Python -v

# Go, direct call edges against this repository. Opt-in: it indexes the tree.
SCOUTER_IMPACT_SELF=1 CGO_ENABLED=0 go test ./tests/ -run TestImpactPrecision_GoIndex -v
```

Both build the CLI from the current checkout and index into a throwaway database through `SCOUTER_CONFIG`. Neither needs a network or a service.

## The corpus

`tests/fixtures/impact_truth/python/` — five files, each carrying a case that separates a correct answer from a plausible one:

| File | What it adds |
| --- | --- |
| `core.py` | Defines `transform`, `scale`, `boost`; `pipeline` calls `scale(transform(i))`. |
| `uses_direct.py` | Calls the same two functions with a plain `from core import scale, transform`. |
| `uses_alias.py` | Reaches `boost` through an import alias: `import core as c` then `c.boost(i)`. |
| `uses_reference.py` | Never calls `boost`; passes it as a value to `map(boost, ...)` and to `functools.partial(boost)`. |
| `decoys.py` | Defines a second `transform` in its own module, mentions `"transform"` inside a string, and names it in a comment. |

`truth.json` next to the corpus is the answer key, written by reading those files: what a human would name as a caller for each query. The index is never read to build it.

## The measurement

One run on the environment below:

| Query | What the answer should be | What Scouter reports | Correct | Missed | Extra |
| --- | --- | --- | ---: | ---: | ---: |
| `scale` | `pipeline`, `direct` | `pipeline`, `direct` | 2 | 0 | 0 |
| `boost` | `aliased`, `via_map`, `via_partial` | *(nothing)* | 0 | 3 | 0 |
| `transform` | refuse: the name means two different functions | 3 callers, one of them from the other module | — | — | — |

| Aggregate | Value |
| --- | ---: |
| Precision | **1.00** |
| Recall | **0.40** |
| True positives | 2 |
| False positives | 0 |
| False negatives | 3 |

## The Go measurement

Go is the primary language, so it gets its own number — measured one layer down, at the direct call edges the indexer writes, because the command answers with the transitive blast radius rendered as fully-qualified names and no hand-written key of direct callers can be compared against that. The answer key is not written out in a file: the oracle parses the tracked Go sources with `go/parser` and walks the AST. A comment is not a call expression, a string literal is not a call expression, and a declaration contains no call to itself, so none of them can be miscounted; each call expression is attributed to the top-level function whose body contains it, which means a call inside a closure belongs to the function that declares the closure. A call in a package-level initializer has no containing function and is not a caller. The index is never opened.

| Query | Call sites in source | Call edges in index | Correct | Missed | Extra |
| --- | ---: | ---: | ---: | ---: | ---: |
| `SanitizeFTS` | 5 | 5 | 5 | 0 | 0 |
| `ValidatePath` | 29 | 29 | 29 | 0 | 0 |

| Aggregate | Value |
| --- | ---: |
| Precision | **1.00** |
| Recall | **1.00** |
| Edges | 34 |

Every direct call edge in this repository for these two symbols is present and none is invented, including calls made from inside closures, from test files, and from methods. The contrast with the Python number is the finding, and it is not a language ranking: it separates two mechanisms. Go's edges come from `go/packages`, which type-checks the module and knows what a call resolves to. Everything else comes from a tree-sitter pass that records the shape of a call expression, so a reference that never appears as the head of a call — an aliased call, a callback passed to `map`, a `functools.partial` — has no edge to record.

Precision 1.00 here means Scouter never invented a caller on this corpus: it did not treat the string in `mention` as a call, and it did not confuse `decoys.transform` with `core.transform` once the ambiguity was refused. Recall 0.40 means it found two of the five callers that exist.

The three misses share a cause: every one of them is a reference that is not a direct call expression. A call through an import alias, a function passed to `map`, and a function passed to `partial` all reach the target without the callee name appearing as the head of a call node. The parser records direct call shapes.

## The defect this measurement found

`impact transform` used to answer. Two different functions in two different modules share that name, and the resolver deduplicated candidates by the rendered fully-qualified name. Python symbols carry no package path, so both definitions rendered as `transform`, collapsed into one candidate, and the engine then matched calls by that bare name across the corpus: the answer merged two call graphs and reported `caller_of_local` from `decoys.py` as a caller of `core.transform`.

The resolver now keys candidates on the definition — the fully-qualified name together with the file — and an ambiguous short name is refused with both files named in the error. The same query now exits non-zero and says so. That is the difference between a confident wrong answer and an honest refusal, and it was invisible from the speed numbers.

## What this does not measure

- **The Go answer the command gives.** The figures above measure direct edges at the index layer. They say nothing about the transitive expansion to depth 5 that `impact` prints, which is a different claim: it can include callers of callers that would never be affected.
- **The rest of Go.** Two symbols and 34 edges in one repository. Wide enough to be falsifiable, not wide enough to be a survey; the opt-in run is the instrument for widening it.
- **Recall beyond Python.** One language, five files, three queries. This is an instrument that shows where misses come from, not a survey.
- **Transitivity quality.** Whether depth-5 expansion adds callers that would never be affected is a separate question from whether the direct edges exist.
- **Anything about speed.** See `docs/benchmarks/results.md`.

## Environment

| | |
| --- | --- |
| Commit | `main` after `4ddac89`, with the ambiguity fix applied |
| Go | `go1.27.1-X:nodwarf5 linux/amd64` |
| CPU | AMD Ryzen 5 5600GT, 12 procs |
| Corpus | 5 Python files for the command-level run; this repository for the Go edge run |
| Database | throwaway, `SCOUTER_CONFIG` pointing at a temp path |

## The ratchet

`known_false_negatives` and `known_false_positives` in `truth.json` record what is wrong today so the measurement can be a CI gate instead of a permanently red test. Both lists are ratchets in two directions: a caller recorded there that stops appearing makes the test fail as *stale* and has to be deleted, so the record cannot outlive the defect it documents. Fixing any of the three misses forces the corresponding entry out of the file.
