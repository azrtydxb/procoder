# A checksum mismatch is refused, but it is not special

Status: done 2026-08-25
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

- [x] A checksum mismatch under a hook exits 0 and executes nothing; the
      same mismatch under a command exits non-zero.
- [x] Neither leaves a file at the cache path, so the next run retries
      cleanly rather than finding a bad binary already there.

## Evidence

- `TestAChecksumMismatchIsNeverExecuted`, two subtests: under a hook the
  launcher exits 0, under a command it exits 1, and in neither case does
  the tampered binary run — the test fails outright if its output appears.
- Neither leaves a file at the cache path, so the next run retries cleanly
  instead of finding a bad binary already there.
- proved by: removing the `[ "$got" = "$want" ]` comparison — the corrupt
  download is installed and executed, and both subtests see its output.
