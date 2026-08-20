# Create internal/releases/compare.go — Semver comparison

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Implement semver comparison for procoder's version strings. Prodetector uses N.N.N format (e.g., 0.28.1) with an optional `v` prefix.

## Acceptance criteria

- [ ] `internal/releases/compare.go` created
- [ ] `Semver` struct: Major, Minor, Patch int
- [ ] `Parse(ver string) Semver` — strips `v` prefix, parses N.N.N, returns empty for invalid strings
- [ ] `Compare(current, latest string) int` returns -1/0/1 (current/newer, equal, latest/newer)
- [ ] `ShouldWarn(current, latest string) bool` — true only if current is a release version and latest > current
- [ ] `dev` string is treated as equal (no version known yet)
- [ ] Empty/invalid strings are treated as equal
- [ ] Comparison order: Major → Minor → Patch
- [ ] Tests cover: identical, patch bump, minor bump, major bump, dev prefix, invalid, empty
- [ ] `go test ./internal/releases/...` passes

## Evidence
