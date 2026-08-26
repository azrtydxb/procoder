# The hot path stays exactly as fast as it is now

Status: done 2026-08-25
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

- [x] With the binary already cached, the launcher makes no network call:
      asserted by running it with fetching sabotaged and requiring
      success.

## Evidence

- `TestACachedBinaryIsRunWithoutAskingTheNetwork`: with a binary present
  the launcher execs it and the stub server records **zero** requests.
  Counting requests proves the fetch was never reached, rather than merely
  that it was quick.
- The cached branch is the first thing after the platform arms, so it
  costs one `[ -x ]` test and an `exec` — what it cost when the binary was
  committed.
- proved by: moving the cached-binary check below the fetch — the request
  count becomes one.
