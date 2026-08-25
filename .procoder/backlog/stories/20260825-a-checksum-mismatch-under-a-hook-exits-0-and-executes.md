# A checksum mismatch is refused, but it is not special

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

A file whose hash does not match the manifest is discarded and never
executed. That much is absolute.

What is not absolute is the exit code. An earlier draft made a mismatch
refuse even under a hook, reasoning that an unverified binary is worse
than no binary. The reasoning is right and the conclusion does not follow:
the protection is not executing the file, which happens either way, so
failing the hook adds no safety and takes the session with it.

The ordinary split applies — hook warns, command refuses — and nothing is
left at the cache path either way.

## Acceptance criteria

- [ ] A checksum mismatch under a hook exits 0 and executes nothing; the
      same mismatch under a command exits non-zero.
- [ ] Neither leaves a file at the cache path, so the next run retries
      cleanly rather than finding a bad binary already there.

## Evidence
