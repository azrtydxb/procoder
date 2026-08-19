# `adr check` refuses on an empty Decision section, an unknown status, a dangling supersede reference, and a duplicated number — each named — and passes on a valid set.

Status: done 2026-08-19
Created: 2026-08-19
Epic: adr
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The adr controller catches every refusal class.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `adr check` refuses on an empty Decision section, an unknown status, a dangling supersede reference, and a duplicated number — each named — and passes on a valid set.

## Evidence

- TestCheckCatchesEveryRefusalClass: empty Decision, status wip, dangling superseded-by-0099, duplicated 0004, unreadable file — all named in one run, Run exits 1; TestCheckPassesValidSetAndCountsProposed green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
