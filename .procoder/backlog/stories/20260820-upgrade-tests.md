# Add tests for releases package

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Create comprehensive tests for the `internal/releases` package covering release fetching, semver comparison, and upgrade flow.

## Acceptance criteria

- [x] `internal/releases/releases_test.go` tests:
  - `Latest()` returns the first release's tag from a mock server
  - `Latest()` panics/retires when network is down (returns error, not panic)
  - `Latest()` returns empty string on 404 (no releases)
  - Timeout is enforced (request does not hang)
- [x] `internal/releases/compare_test.go` tests:
  - Same versions → Compare returns 0
  - Patch bump → Compare returns 1
  - Minor bump → Compare returns 1
  - Major bump → Compare returns 1
  - Current newer → Compare returns -1
  - Dev version → Compare returns 0
  - Invalid strings → Compare returns 0
  - `ShouldWarn` returns false for dev, equal, or older
  - `ShouldWarn` returns true only for strictly newer release
- [x] `internal/releases/upgrade_test.go` tests:
  - Download succeeds → temp file deleted, binary updated
  - Download fails → temp file cleaned up, binary unchanged
  - Current version > latest → refuses download (downgrade protection)
- [x] `go test ./internal/releases/...` — all tests pass

## Evidence

- `go test ./internal/releases/` green: 11 tests over comparison, the GitHub query, and the upgrade controller.
- Every network path is served by an httptest stub — no test reaches api.github.com, so the suite does not depend on egress, on rate limits, or on what the newest release happens to be today.
- No test overwrites the binary running it: the controller's path resolution is injected, so the upgrade writes to a scratch directory.
- Failure paths carry as many tests as the happy path: rate limiting, 404, HTML, no tag, timeout, missing platform asset, failed download, declined consent, downgrade, dev build.
- Found live rather than by test, then pinned: /dev/null is a character device, so the terminal check reported a human on `version --check < /dev/null`. TestDevNullIsNotATerminal now covers it, alongside a pipe and nil.
