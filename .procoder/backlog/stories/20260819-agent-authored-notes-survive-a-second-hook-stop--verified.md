# Agent-authored notes survive a second `hook stop` — verified by a test that writes notes, re-runs, and asserts they remain while the facts block updates.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

The agent's notes are never dropped.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Agent-authored notes survive a second `hook stop` — verified by a test that writes notes, re-runs, and asserts they remain while the facts block updates.

## Evidence

- TestAgentNotesSurviveTheNextStopWhileFactsUpdate and TestHandDeletedMarkersRewriteTheFileAndKeepTheNotes: even with the markers hand-deleted, everything from the Notes heading down is carried.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
