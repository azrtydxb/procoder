# `adr new` prints 0001 for an empty repo and 0003 when 0002 exists; the printed file carries all three sections and today's date; nothing is written by the binary.

Status: done 2026-08-19
Created: 2026-08-19
Epic: adr
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

New numbers records and writes nothing.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `adr new` prints 0001 for an empty repo and 0003 when 0002 exists; the printed file carries all three sections and today's date; nothing is written by the binary.

## Evidence

- TestNewNumbersFromOneAndWritesNothing + TestNewNumbersAfterExisting (0003 after 0002). Live: `procoder adr new` printed 0001 with all three sections; ADR 0001 then written by the agent and `adr check` exits 0.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
