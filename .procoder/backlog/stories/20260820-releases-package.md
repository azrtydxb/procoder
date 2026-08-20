# Create internal/releases/releases.go — GitHub release query

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Create the `internal/releases/releases` package that queries GitHub's releases API and returns the latest tagged release.

## Acceptance criteria

- [x] `internal/releases/releases.go` created
- [x] `Release` struct with `TagName` and `Assets []ReleaseAsset`
- [x] `ReleaseAsset` struct with `URL` and `Name`
- [x] `Latest(root string, timeout time.Duration) (string, error)` fetches from `api.github.com/repos/azrtydxb/procoder/releases`
- [x] Uses `http.Client{Timeout: timeout}` — hard deadline enforced
- [x] Returns the first (most recent) release's `tag_name`
- [x] Handles errors gracefully: network failure → error (not panic), non-200 → error, no tag → error
- [x] Package compiles with stdlib only (no external deps)
- [x] Tests: mock HTTP server returns correct tag; timeout is enforced; 404 handled

## Evidence

- internal/releases/releases.go: `Latest(timeout)` queries `/repos/azrtydxb/procoder/releases/latest` over stdlib net/http, with the API-version header and no token — nothing about who is asking leaves the machine.
- The body is read through io.LimitReader(1MB): a response that never ends must not become memory that never stops growing.
- TestLatestReadsTheTagAndItsAssets against an httptest stub; no test reaches api.github.com, so the suite passes on a plane and under rate limiting.
- TestAnUnanswerableCheckIsNeverUpToDate covers 403 rate limiting, 404, HTML instead of JSON, and a 200 naming no tag — each returns an error whose text says what happened, and Check never warns on any of them.
- TestTheTimeoutIsEnforced pins the one-second cap against a deliberately slow server (N-02).
- Live: a build stamped 0.9.0 asked the real GitHub and reported 1.0.2.
