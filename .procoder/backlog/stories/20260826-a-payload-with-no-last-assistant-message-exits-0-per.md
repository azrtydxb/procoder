# A payload with no `last_assistant_message` exits 0, per `TestNoMessageIsNotEvidence`; fails if an absent field is treated as an empty ask.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

An older host, another host, or a malformed event. An absent field is not evidence that a decision was buried.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A payload with no `last_assistant_message` exits 0, per `TestNoMessageIsNotEvidence`; fails if an absent field is treated as an empty ask.

## Evidence

`TestNoMessageIsNotEvidence`. Recorded honestly: the empty-message guard is an early return, not the protection — an empty string matches no ask phrase anyway, and removing it fails nothing. The test pins the behaviour so a future detector that treats absence as something fails here.
