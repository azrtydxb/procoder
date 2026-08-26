# Every refusal introduced here carries a fix in its text, asserted by `TestEveryRefusalNamesTheFix`, which reads the messages rather than trusting inspection.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A checker that refuses without a route out is a gate people learn to route around, which is how `--no-verify` becomes muscle memory.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Every refusal introduced here carries a fix in its text, asserted by `TestEveryRefusalNamesTheFix`, which reads the messages rather than trusting inspection.

## Evidence

`TestEveryRefusalNamesTheFix` reads the message text rather than trusting inspection, and was named in the spec's criterion before it existed. Killed by removing the trailing advice from either message.
