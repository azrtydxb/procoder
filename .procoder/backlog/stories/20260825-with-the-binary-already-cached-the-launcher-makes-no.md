# The hot path stays exactly as fast as it is now

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

The launcher runs on every session start, every Bash call and every
write. Whatever the fetch costs, it must be paid once and never again.

With a binary cached, the launcher execs it directly — no network call, no
subprocess, no extra work of any kind. Asserted by sabotaging the fetch
entirely (`PROCODER_NO_FETCH`) and requiring the run to succeed anyway,
which proves the fetch was never reached rather than merely that it was
fast.

## Acceptance criteria

- [ ] With the binary already cached, the launcher makes no network call:
      asserted by running it with fetching sabotaged and requiring
      success.

## Evidence
