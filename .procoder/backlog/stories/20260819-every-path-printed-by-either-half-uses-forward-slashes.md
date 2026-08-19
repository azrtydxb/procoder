# Every path printed by either half uses forward slashes, asserted by a test that builds a nested fixture path and rejects any backslash in the output.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Windows-safe output.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Every path printed by either half uses forward slashes, asserted by a test that builds a nested fixture path and rejects any backslash in the output.

## Evidence

- TestEveryPrintedPathUsesForwardSlashes; PathError messages are stripped to op+errno so an OS path can never reach output.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
