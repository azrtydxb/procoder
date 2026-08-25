# No half-written binary, ever

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: -

## Description

Two hooks can fire at once, and a download can be interrupted. Neither
may leave a partial file where the launcher expects a working binary — the
next run would exec it.

The download lands on a temporary path and is renamed into place only once
it is complete and verified. A racing second process either sees no binary
or a whole one; there is no third state.

## Acceptance criteria

- [ ] A download that is interrupted leaves no file at the cache path.
- [ ] Two launchers racing on first run both end with a working binary and
      neither observes a partial file.

## Evidence
