# `procoder hook stop` writes `.procoder/state/handoff.md` containing the facts block and today's HEAD.

Status: done 2026-08-19
Created: 2026-08-19
Epic: session-continuity
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

The handoff note is written.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder hook stop` writes `.procoder/state/handoff.md` containing the facts block and today's HEAD.

## Evidence

- Live: `echo '{}' | procoder hook stop` exited 0 silently and wrote .procoder/state/handoff.md with the facts block, generated timestamp, HEAD 10b1491, branch, dirty count and the sprint. TestStopWritesTheFactsBlockWithHead green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
