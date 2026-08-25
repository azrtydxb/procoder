# The checks that guarded committed binaries go with them

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 020-the-binaries-leave-the-tree-and-ci-builds-them

## Description

CI's reproducibility job answered one question — do these committed
binaries rebuild from the source they were committed beside — and there
will be no committed binaries and no committer.

The release controller's shipped-binary version check goes the same way:
it was written a day earlier against exactly the failure this epic makes
impossible.

Both are removed with their tests, rather than left asserting behaviour
that no longer exists. What is being spent, deliberately, is third-party
verifiability: with nothing to rebuild against, nobody outside CI can
independently confirm the published bytes. That is the trust already
extended to every CI-published artifact, and it is recorded rather than
discovered.

## Acceptance criteria

- [x] CI runs no reproducibility job, and `procoder release` no longer
      checks a shipped binary.
- [x] The tests that covered both are removed rather than left asserting
      behaviour that is gone.

## Evidence

- The "the committed binaries rebuild from their source" job is removed
  from `ci.yml` (35 lines). It answered one question — do these committed
  binaries rebuild from the commit that carried them — and there are no
  committed binaries and no committer.
- `staleDist`, `execReason`, `lastLine` and `DistDir` are removed from
  `internal/release`, along with the six tests that covered them, rather
  than left asserting behaviour that no longer exists. That check was
  written a day earlier against exactly the failure this sprint makes
  impossible.
- A consequence worth naming: the test job takes a shallow checkout again.
  Its `fetch-depth: 0` existed only so the reproducibility step could
  rebuild `dist/` at an older commit. Verified before removing it that no
  test needs the checkout's history — every test that touches git builds
  its own repository in `t.TempDir()`.
- What is spent is recorded in ADR 0004: third-party verifiability. With
  nothing committed there is nothing to rebuild against, and the
  provenance is the workflow run.
