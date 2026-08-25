# Nothing unverified is executed

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: -

## Description

The rule the fetch exists to uphold. A binary is executed only after
its sha256 matches the line for its platform in the published manifest.

A `SHA256SUMS` that downloads but carries no line for this platform is a
failed verification, never a pass — the absence of a checksum is not the
absence of a problem.

## Acceptance criteria

- [ ] A binary whose sha256 does not match the manifest is not executed
      and is not left behind, for a hook as much as for a command.
- [ ] A `SHA256SUMS` with no line for this platform is treated as a failed
      verification rather than as nothing to check.

## Evidence
