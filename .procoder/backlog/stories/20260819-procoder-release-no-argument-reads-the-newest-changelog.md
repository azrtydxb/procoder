# `procoder release` (no argument) reads the newest changelog version and reports the checklist for it.

Status: done 2026-08-19
Created: 2026-08-19
Epic: release-discipline
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Bare release audits the newest version.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder release` (no argument) reads the newest changelog version and reports the checklist for it.

## Evidence

- TestNoArgumentReadsNewestChangelogVersion picks 0.2.0 over 0.1.0; missing changelog exits 2.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
