# `--name` combined with `[paths...]` and `--coverage` in one invocation produces both the narrowed package list and a coverage number, pinned by a test over the constructed argv per ecosystem.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Filter, paths and coverage compose.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `--name` combined with `[paths...]` and `--coverage` in one invocation produces both the narrowed package list and a coverage number, pinned by a test over the constructed argv per ecosystem.

## Evidence

- TestGoArgsCarryFilterPathsAndCoverage and TestPytestArgsCarryFilterPathsAndCoverage assert the constructed argv per ecosystem.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
