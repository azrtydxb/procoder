# CI builds the binaries; nobody else ever does

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: -

## Description

The release job copies files a person committed. It builds them
instead, from the tagged source, for all five targets, and publishes them
with a `SHA256SUMS` generated in that same job.

The build script already cross-compiles all five, so this is a change of
caller rather than of mechanism.

And the job must fail rather than publish a release with no assets. An
empty release looks finished and breaks every installer downstream, which
is worse than a red job somebody has to look at.

## Acceptance criteria

- [ ] The release job builds all five targets from the tagged source and
      attaches them with a `SHA256SUMS` generated in that job.
- [ ] The job fails rather than publishing a release with no assets.

## Evidence
