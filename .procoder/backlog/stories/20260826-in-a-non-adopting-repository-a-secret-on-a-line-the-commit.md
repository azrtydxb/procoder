# In somebody else's repository, only the diff is mine

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

The last finding in the report, and the one that most deserved the
`--no-verify` it provoked: a constant named `..._STORE_KEY` on line 4,423
of a file whose diff sat 2,500 lines away. Not written by this commit, not
a credential, and blocking it anyway.

In a repository that has not adopted procoder, the checks that read file
content see only the lines this commit added or changed. Four thousand
lines somebody else wrote are not mine to answer for.

## Acceptance criteria

- [x] In a non-adopting repository, a secret on a line the commit did not
      touch produces no finding, and the same secret on a line the commit
      added blocks.

## Evidence

`gitx.NarrowToDiff` + `security.SecretsInDiff`. Both halves proved at the
gate over a 3,000-line fixture with a stubbed scanner:
`TestAPreExistingSecretDoesNotBlockInANonAdoptingRepository` (flagged line
2,500, change at line 2 — exit 0) and
`TestASecretOnALineTheCommitWroteStillBlocks` (same fixture, the commit
writes line 2,500 — exit 1). Unit-level in
`internal/gitx/diffscope_test.go`, including the no-silent-green case:
when the diff cannot be read, every finding is reported.
