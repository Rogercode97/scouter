# Tasks: Security Update

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~20 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

## Phase 1: Go Dependencies Update

- [x] 1.1 In `go.mod`, update the `github.com/go-git/go-git/v6` dependency to the latest version.
- [x] 1.2 Run `go mod tidy` to update `go.sum`.

## Phase 2: GitHub Actions Update

- [x] 2.1 Search through `.github/workflows/*.yml` files and update `actions/github-script` references to `v9` or `@v9`.
- [x] 2.2 Search through `.github/workflows/*.yml` files and update `goreleaser/goreleaser-action` references to `v7` or `@v7`.

## Phase 3: Verification

- [x] 3.1 Run `just test` locally to ensure no tests break from the `go-git` update.
