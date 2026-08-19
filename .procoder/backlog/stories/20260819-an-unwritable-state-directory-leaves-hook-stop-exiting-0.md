# An unwritable state directory leaves `hook stop` exiting 0 with no output — verified by a test.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

A failed handoff never breaks a session.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] An unwritable state directory leaves `hook stop` exiting 0 with no output — verified by a test.

## Evidence

- TestUnwritableStateDirectoryExitsZeroSilently (skipped on Windows and as root); TestStopToleratesAnEmptyPayload.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
