# A criterion naming `procoder check` or a `Test...` function is accepted.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A rule that accepted only one shape would push people into writing criteria that fit the checker rather than criteria that are true.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A criterion naming `procoder check` or a `Test...` function is accepted.

## Evidence

`TestACriterionNamingItsObservableIsAccepted` covers all three shapes — a command, a test name, a file. Killed by removing the `procoder ` alternative from `observableRe`.
