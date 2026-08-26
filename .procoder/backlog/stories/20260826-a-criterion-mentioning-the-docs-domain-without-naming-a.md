# A criterion mentioning the docs domain without naming a built index is refused, and the refusal names the index as the missing prerequisite; the same criterion with `procoder index build` named is accepted.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

The criteria that were green whatever the code did. Without a built index the docs domain reports `public surface NOT computed` and never reaches a finding, so a criterion about a documentation obligation on an unindexed fixture cannot fail.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A criterion mentioning the docs domain without naming a built index is refused, and the refusal names the index as the missing prerequisite; the same criterion with `procoder index build` named is accepted.

## Evidence

`TestACriterionMissingItsPrerequisiteIsReported` and `TestNamingThePrerequisiteClearsIt` — the second because a rule nobody can satisfy gets the criterion deleted rather than fixed. Killed by removing the docs entry from `prerequisites`, and by removing the `p.names` escape.
