# The handoff note is written on the blocking path too, per `TestTheHandoffNoteSurvivesABlock`; fails if the block returns before the note is written.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

A blocked turn must not also lose the note the next session inherits.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The handoff note is written on the blocking path too, per `TestTheHandoffNoteSurvivesABlock`; fails if the block returns before the note is written.

## Evidence

The note is written before any blocking decision is taken. `TestTheHandoffNoteSurvivesABlock` asserts the file exists after a blocking stop, and fails the fixture first if the stop did not block — so it cannot pass by never blocking.
