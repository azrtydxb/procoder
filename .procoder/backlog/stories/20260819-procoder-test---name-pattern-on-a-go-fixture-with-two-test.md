# `procoder test --name <pattern>` on a Go fixture with two test functions runs only the matching one, verified by the reported counts, and exits 0.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

The filter narrows a real run.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test --name <pattern>` on a Go fixture with two test functions runs only the matching one, verified by the reported counts, and exits 0.

## Evidence

- TestGoNameRunsOnlyTheMatchingTest (live fixture). Live on this repo: `test --name TestSlugifyCapsLongTitles` matched 2 packages.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
