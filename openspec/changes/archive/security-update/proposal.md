# Proposal: Security Update

## Intent

Update Dependabot vulnerabilities in go.mod (specifically go-git/v6 to latest) and .github/workflows/ (github-script-9, goreleaser-action-7) to address security flaws and technical debt.

## Scope

### In Scope
- Update `go-git/v6` to latest version in `go.mod`.
- Update `actions/github-script` to `v9` in `.github/workflows/*`.
- Update `goreleaser/goreleaser-action` to `v7` in `.github/workflows/*`.

### Out of Scope
- Updating other unrelated dependencies.
- Refactoring internal code unless strictly required by the dependency updates.

## Capabilities

### New Capabilities
None

### Modified Capabilities
None

## Approach

- Modify `go.mod` to require the latest stable version of `github.com/go-git/go-git/v6` and run `go mod tidy` to update `go.sum`.
- Search through `.github/workflows/` and update references of `actions/github-script` to `@v9` (or `v9`).
- Search through `.github/workflows/` and update references of `goreleaser/goreleaser-action` to `@v7` (or `v7`).
- Verify the updates by running the test suite locally.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `go.mod` / `go.sum` | Modified | Update `go-git/v6` dependency |
| `.github/workflows/` | Modified | Update GitHub Action versions |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Breaking API changes in `go-git` | Low | Run full `just test` suite after update |
| Actions failing with new major versions | Low | Rely on CI pipeline checks |

## Rollback Plan

Revert the specific commit or PR that updates these dependency versions.

## Dependencies

- None

## Success Criteria

- [ ] `go test ./...` passes locally.
- [ ] CI pipeline successfully completes all workflow runs.
- [ ] Dependabot alerts for these specific dependencies are resolved.
