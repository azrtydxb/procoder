# The same payload, with an unanswered decision recorded, exits 0 and says nothing, per `TestARecordedDecisionIsSilent`; fails if the recorded-decision check is dropped.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

The agent did the thing. Being told about it anyway is how a check stops being read.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The same payload, with an unanswered decision recorded, exits 0 and says nothing, per `TestARecordedDecisionIsSilent`; fails if the recorded-decision check is dropped.

## Evidence

`ask.PendingDecisions` — a narrow query rather than `Pending`, which runs git, the lint pass and the secret scan. A hook firing at the end of every turn under a ten-second timeout cannot afford that. Proved by `TestARecordedDecisionIsSilent`; killed by removing the recorded-decision branch.
