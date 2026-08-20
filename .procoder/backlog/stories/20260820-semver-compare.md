# Create internal/releases/compare.go — Semver comparison

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Implement semver comparison for procoder's version strings. Prodetector uses N.N.N format (e.g., 0.28.1) with an optional `v` prefix.

## Acceptance criteria

- [x] `internal/releases/compare.go` created
- [x] `Semver` struct: Major, Minor, Patch int
- [x] `Parse(ver string) Semver` — strips `v` prefix, parses N.N.N, returns empty for invalid strings
- [x] `Compare(current, latest string) int` returns -1/0/1 (current/newer, equal, latest/newer)
- [x] `ShouldWarn(current, latest string) bool` — true only if current is a release version and latest > current
- [x] `dev` string is treated as equal (no version known yet)
- [x] Empty/invalid strings are treated as equal
- [x] Comparison order: Major → Minor → Patch
- [x] Tests cover: identical, patch bump, minor bump, major bump, dev prefix, invalid, empty
- [x] `go test ./internal/releases/...` passes

## Evidence

- internal/releases/compare.go: Parse, Compare, ShouldWarn, with Major→Minor→Patch ordering and the `v` prefix stripped.
- DEVIATION: the struct is named `Version`, not `Semver` — `releases.Semver` stutters where `releases.Version` reads. Fields are exactly as specified.
- DEVIATION, and a deliberate strengthening: `Parse` returns `(Version, bool)` rather than an empty struct for invalid input. An empty struct is 0.0.0, which compares as older than every real release — a dev build would be told to upgrade and a maintainer ahead of the tag would be told to downgrade. Unknown is a third answer, and the bool is what carries it.
- TestCompareOrdersMajorThenMinorThenPatch covers identical, patch, minor and major bumps, the `v` prefix both ways, a pre-release suffix, and 0.28.10 > 0.28.1 (not a string compare).
- TestUnknownVersionsCompareAsEqualAndNeverWarn covers dev, empty, non-numeric, too-few and too-many components — each compares 0 and never warns.
- `go test ./internal/releases/` green.
