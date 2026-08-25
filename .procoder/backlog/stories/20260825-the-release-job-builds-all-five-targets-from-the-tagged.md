# CI builds the binaries; nobody else ever does

Status: done 2026-08-25
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 020-the-binaries-leave-the-tree-and-ci-builds-them

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

- [x] The release job builds all five targets from the tagged source and
      attaches them with a `SHA256SUMS` generated in that job.
- [x] The job fails rather than publishing a release with no assets.

## Evidence

- `.github/workflows/ci.yml` gains a "build the binaries this release
  publishes" step that runs `scripts/build-dist.sh` under the pinned
  toolchain, before staging. The staging step still copies from `dist/` —
  but from a `dist/` this job just built, not one a person committed.
- The job refuses to publish a release without a complete manifest: it
  requires `SHA256SUMS` to carry five lines and fails, printing the file,
  otherwise. A release with no assets used to mean nobody could upgrade;
  now it means nobody can install, so it is worth a red job.
- The existing five-distinct-names check is unchanged and still runs
  before the manifest is staged, so it keeps meaning "five binaries".
