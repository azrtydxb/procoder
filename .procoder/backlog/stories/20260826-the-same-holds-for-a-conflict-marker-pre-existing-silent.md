# The same narrowing for conflict markers

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

Conflict markers read file content too, so they narrow the same way. A
marker this commit introduced is this commit's problem; one that was
already in a file it merely touched is not.

The two content checks are narrowed together rather than one at a time,
because a rule that applies to one and not the other is a rule nobody can
predict.

## Acceptance criteria

- [ ] In a non-adopting repository a pre-existing conflict marker is
      silent, and one introduced by this commit blocks.

## Evidence
