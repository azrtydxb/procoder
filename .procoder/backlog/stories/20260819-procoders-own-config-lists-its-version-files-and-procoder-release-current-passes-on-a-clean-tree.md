# procoder's own config lists its version files and `procoder release <current>` passes on a clean tree.

Status: done 2026-08-19
Created: 2026-08-19
Epic: release-discipline
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

This repo dogfoods the checklist.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] procoder's own config lists its version files and `procoder release <current>` passes on a clean tree.

## Evidence

- .procoder/config.toml lists the nine version files under [release]; `procoder release <current>` verified at release time on the clean tree (see the 0.28.0 release evidence in CHANGELOG/PR).

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
