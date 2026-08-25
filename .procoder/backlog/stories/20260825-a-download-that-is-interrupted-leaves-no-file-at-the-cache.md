# No half-written binary, ever

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

Two hooks can fire at once, and a download can be interrupted. Neither
may leave a partial file where the launcher expects a working binary — the
next run would exec it.

The download lands on a temporary path and is renamed into place only once
it is complete and verified. A racing second process either sees no binary
or a whole one; there is no third state.

## Acceptance criteria

- [x] A download that is interrupted leaves no file at the cache path.
- [x] Two launchers racing on first run both end with a working binary and
      neither observes a partial file.

## Evidence

- `TestTwoLaunchersRacingBothEndWithAWorkingBinary`: two launchers race on
  a fresh install, both end with a working binary, and the file at the
  cache path is the whole downloaded payload.
- **Recorded honestly:** this proves the race is survivable, NOT that the
  install is atomic. Swapping the `mv` for a `cp` leaves the test passing,
  because copying a small file wins a two-way race almost every time and
  the unsafe window is microseconds wide. That mutation was run and did
  exactly that. The rename is kept because it costs nothing and is the
  standard answer, and the limit is written into the test rather than
  dressed up as a proof it does not have.
