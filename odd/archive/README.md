# Recovered archive: the OpenSpec change records

## Provenance

These files were written under OpenSpec, not ODD, and lived in `openspec/`:

```
openspec/changes/archive/<change>/    ->  odd/archive/changes/<change>/
openspec/changes/predictive-testing/  ->  odd/archive/changes/predictive-testing/
openspec/specs/<capability>/          ->  odd/specs/<capability>/
```

They were deleted from the default branch and recovered from commit `e7a6b90` ("v1.0.0 release
(Hakai Edition)", 2026-05-16), which still carried the complete tree. That tree also survives on
the branches `docs/cognitive-design` (84 files) and `refactor/analysis-slicing` (68 files).

Excluded on recovery as tooling state rather than content: `openspec/config.yaml`,
`openspec/state.yaml`, `openspec/tasks.md`, and two session logs inside `predictive-testing`
(`read_log.md`, `tmp_read.txt`).

## How much of this is trustworthy

Nothing here was re-run during recovery. The reason for caution is specific, not general:

- **The same era produced a verification record that proved false.** The archived PASS for the
  v1.0.0 hardening declared `SafeCommand` universally hardened; the argument-injection gap it
  claimed to close was still open, and was closed on 2026-10-17 in PRs #58 and #61.
- **Six `verdict.md`, and only two of them with a `verify-report.md`.** Four changes
  (`v2-omniscience`, `impact-analysis`, `interface-tracing`, `public-hardening`) declare success
  with no verification record in the archive at all.
- **A spot check of `public-hardening/verdict.md`**, whose four achievements were checked against
  the tree on 2026-10-17: three still hold — `SanitizeFTS` exists in
  `internal/utils/security.go` with eight call sites, `ValidatePath` has tests in four files, and
  `LICENSE` is MIT — and the fourth ("global 500-row limit across all MCP handlers") is
  overstated rather than false: `limit = 500` exists in `internal/store/store.go`, while the MCP
  layer has its own separate caps.

So the verdicts here are **declarations, not evidence**. Every `verdict.md` and `verify-report.md`
carries a `recovered-from-openspec` marker at the top for that reason, and
`predictive-testing/` was never archived at all — it is an unfinished change, not a closed one.

If a claim in this archive matters to a current decision, re-run it against the code instead of
citing the file.
