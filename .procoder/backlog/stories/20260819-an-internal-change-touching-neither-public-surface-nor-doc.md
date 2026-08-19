# An internal change touching neither public surface nor doc-mentioned files raises nothing.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

No obligation without a trigger.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] An internal change touching neither public surface nor doc-mentioned files raises nothing.

## Evidence

- TestInternalChangeRaisesNothing green; the index-diff implementation was chosen precisely so an ordinary edit does not fire.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
