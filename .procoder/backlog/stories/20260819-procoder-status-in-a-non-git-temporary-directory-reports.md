# `procoder status` in a non-git temporary directory reports the git-derived lines as unknown with a reason and still exits 0.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Unknown with a reason, never a default.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder status` in a non-git temporary directory reports the git-derived lines as unknown with a reason and still exits 0.

## Evidence

- TestReportOutsideARepoIsUnknownWithTheReason and TestUnreadableTodoDirectoryIsUnknownNotZero.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
