# A failed fetch is remembered, briefly

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

Hooks fire on every write and every Bash call. If each one retried a
failing download, an offline machine would make dozens of failing network
calls a minute on the hot path.

The failure is recorded beside the cache path with a timestamp, and no
further attempt is made until it ages out. Within that window the launcher
reports the recorded reason without touching the network.

This is a memory, not a silence: every invocation still says what is
wrong. The distinction matters — a quiet launcher would be the failure
this project is built to catch.

## Acceptance criteria

- [ ] A second invocation inside the failure window makes no network call
      and still reports the reason, asserted by counting requests against
      a stub server.
- [ ] After the window ages out, the next invocation tries again.

## Evidence
