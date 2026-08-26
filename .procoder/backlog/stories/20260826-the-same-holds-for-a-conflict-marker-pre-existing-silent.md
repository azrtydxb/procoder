# The same narrowing for conflict markers

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

Conflict markers read file content too, so they narrow the same way. A
marker this commit introduced is this commit's problem; one that was
already in a file it merely touched is not.

The two content checks are narrowed together rather than one at a time,
because a rule that applies to one and not the other is a rule nobody can
predict.

## Acceptance criteria

- [x] In a non-adopting repository a pre-existing conflict marker is
      silent, and one introduced by this commit blocks.

## Evidence

`TestAPreExistingConflictMarkerIsSilentInANonAdoptingRepository` (upstream
committed the marker; this commit appends one line — exit 0) and
`TestUniversalStillBlocksOnAConflictMarkerTheCommitWrote` (exit 1).
