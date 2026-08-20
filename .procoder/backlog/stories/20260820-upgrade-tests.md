# Add tests for releases package

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Create comprehensive tests for the `internal/releases` package covering release fetching, semver comparison, and upgrade flow.

## Acceptance criteria

- [ ] `internal/releases/releases_test.go` tests:
  - `Latest()` returns the first release's tag from a mock server
  - `Latest()` panics/retires when network is down (returns error, not panic)
  - `Latest()` returns empty string on 404 (no releases)
  - Timeout is enforced (request does not hang)
- [ ] `internal/releases/compare_test.go` tests:
  - Same versions → Compare returns 0
  - Patch bump → Compare returns 1
  - Minor bump → Compare returns 1
  - Major bump → Compare returns 1
  - Current newer → Compare returns -1
  - Dev version → Compare returns 0
  - Invalid strings → Compare returns 0
  - `ShouldWarn` returns false for dev, equal, or older
  - `ShouldWarn` returns true only for strictly newer release
- [ ] `internal/releases/upgrade_test.go` tests:
  - Download succeeds → temp file deleted, binary updated
  - Download fails → temp file cleaned up, binary unchanged
  - Current version > latest → refuses download (downgrade protection)
- [ ] `go test ./internal/releases/...` — all tests pass

## Evidence
