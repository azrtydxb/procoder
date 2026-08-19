# Sprint close writes a Retro scaffold; `sprint open` then refuses until the Retro has real content, and proceeds after one sentence is added — verified by a test walking the sequence.

Status: done 2026-08-19
Created: 2026-08-19
Epic: backlog-extensions
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The retro is the price of the next sprint.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Sprint close writes a Retro scaffold; `sprint open` then refuses until the Retro has real content, and proceeds after one sentence is added — verified by a test walking the sequence.

## Evidence

- TestSprintCloseWritesRetroScaffold + TestSprintOpenRefusesUntilRetroIsFilled walk the full sequence. Live: opening a sprint over the fixture's unretroed 001-login-mvp refused naming the file.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
