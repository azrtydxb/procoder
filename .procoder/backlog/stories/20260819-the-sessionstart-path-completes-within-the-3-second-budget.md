# The SessionStart path completes within the 3-second budget on a repository with a built index and an active sprint — verified by a timed test.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Well inside the budget.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The SessionStart path completes within the 3-second budget on a repository with a built index and an active sprint — verified by a timed test.

## Evidence

- TestSessionStartStaysInsideTheBudget and TestReportStaysInsideTheBudget. Measured on this repository (27 open stories, index present, 52 dirty files): 76.6ms against a 3s budget. Git work runs under a deadline; on overrun the git lines drop as a unit with a spoken note.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
