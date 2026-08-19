# `[sprint] retro = "off"` disables the retro gate, verified by test.

Status: done 2026-08-19
Created: 2026-08-19
Epic: backlog-extensions
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The repo can opt out.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[sprint] retro = "off"` disables the retro gate, verified by test.

## Evidence

- TestSprintOpenRetroGateDisabledByConfig: [sprint] retro = "off" in config.toml lets open proceed.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
