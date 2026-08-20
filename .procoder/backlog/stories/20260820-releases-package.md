# Create internal/releases/releases.go — GitHub release query

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Create the `internal/releases/releases` package that queries GitHub's releases API and returns the latest tagged release.

## Acceptance criteria

- [ ] `internal/releases/releases.go` created
- [ ] `Release` struct with `TagName` and `Assets []ReleaseAsset`
- [ ] `ReleaseAsset` struct with `URL` and `Name`
- [ ] `Latest(root string, timeout time.Duration) (string, error)` fetches from `api.github.com/repos/azrtydxb/procoder/releases`
- [ ] Uses `http.Client{Timeout: timeout}` — hard deadline enforced
- [ ] Returns the first (most recent) release's `tag_name`
- [ ] Handles errors gracefully: network failure → error (not panic), non-200 → error, no tag → error
- [ ] Package compiles with stdlib only (no external deps)
- [ ] Tests: mock HTTP server returns correct tag; timeout is enforced; 404 handled

## Evidence
