# `.procoder/state/` appears in the gitignore guidance the docs domain checks.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

The new state directory is ignored.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `.procoder/state/` appears in the gitignore guidance the docs domain checks.

## Evidence

- gitx.gitignoreNeeds gained {".procoder", ".procoder/state/"}; this repo's own .gitignore now carries the line and `procoder check` is clean.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
