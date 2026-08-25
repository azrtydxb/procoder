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

- [ ] CI runs no reproducibility job, and `procoder release` no longer
      checks a shipped binary.
- [ ] The tests that covered both are removed rather than left asserting
      behaviour that is gone.

## Evidence
