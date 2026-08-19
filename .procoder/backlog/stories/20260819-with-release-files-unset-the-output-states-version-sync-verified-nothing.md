# With `[release] files` unset the output states version-sync verified nothing.

Status: done 2026-08-19
Created: 2026-08-19
Epic: release-discipline
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

No silent pass without configuration.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With `[release] files` unset the output states version-sync verified nothing.

## Evidence

- TestEmptyReleaseFilesSaysVerifiedNothing: the version-sync leg says it verified nothing and points at [release] files.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
