# The block policy makes the obligation block the gate; the default report leaves the gate's verdict unchanged — both verified by test.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The caller owns the verdict.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The block policy makes the obligation block the gate; the default report leaves the gate's verdict unchanged — both verified by test.

## Evidence

- TestBlockPolicyIsTheCallersDecision green; [docs] policy parsed in internal/config, default report so no adopter's gate verdict changes on upgrade. Live: this fixture ran under block and denied.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
