# Nothing unverified is executed

Status: done 2026-08-25
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

The rule the fetch exists to uphold. A binary is executed only after
its sha256 matches the line for its platform in the published manifest.

A `SHA256SUMS` that downloads but carries no line for this platform is a
failed verification, never a pass — the absence of a checksum is not the
absence of a problem.

## Acceptance criteria

- [x] A binary whose sha256 does not match the manifest is not executed
      and is not left behind, for a hook as much as for a command.
- [x] A `SHA256SUMS` with no line for this platform is treated as a failed
      verification rather than as nothing to check.

## Evidence

- `TestAChecksumMismatchIsNeverExecuted` for the mismatch, and
  `TestAnAssetWithNoChecksumLineIsNotRun` for a manifest that downloads
  but carries no line for this platform — the absence of a checksum is
  treated as a failed verification, never as nothing to check.
- proved by: treating an empty expected hash as success — an unlisted
  asset is then run unverified. When that guard alone is removed the
  checksum comparison still refuses, with a vaguer message, and the test
  fails on the message: the layers are independent and the accurate one is
  pinned.
