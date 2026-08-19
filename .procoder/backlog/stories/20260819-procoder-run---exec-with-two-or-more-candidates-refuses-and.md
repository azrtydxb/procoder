# `procoder run --exec` with two or more candidates refuses and exits 2; with exactly one non-server candidate it executes the command and exits with 0 on success, 1 on failure.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

A choice is the user's to make.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder run --exec` with two or more candidates refuses and exits 2; with exactly one non-server candidate it executes the command and exits with 0 on success, 1 on failure.

## Evidence

- TestExecRefusesSeveralCandidates: exit 2, both candidates still printed with their evidence.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
