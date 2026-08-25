# A command that cannot run refuses, loudly

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

The other half, and the one that is easy to get wrong by being kind.

`procoder check` that cannot find its binary must NOT exit 0. That is a
silent green sitting in the launcher every other check runs through — the
same defect as `check --staged` exiting 0 having assessed a mistyped
filename, which is what v3.0.0 was largely about.

Whatever went wrong is named: no network, no release for this version, a
checksum that did not match, a directory it could not write.

## Acceptance criteria

- [ ] `launcher.sh check` with no binary and no network exits non-zero and
      names the reason.
- [ ] The reason distinguishes the causes — unreachable, no such release,
      checksum mismatch, unwritable cache — rather than one message for
      all of them.

## Evidence
