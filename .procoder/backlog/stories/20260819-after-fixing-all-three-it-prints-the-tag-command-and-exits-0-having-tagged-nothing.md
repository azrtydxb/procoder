# After fixing all three, it prints the tag command and exits 0, having tagged nothing.

Status: done 2026-08-19
Created: 2026-08-19
Epic: release-discipline
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Ready prints the tag and tags nothing.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] After fixing all three, it prints the tag command and exits 0, having tagged nothing.

## Evidence

- TestReadyPrintsTagCommandAndTagsNothing: after fixes, exit 0, tag command printed, `git tag -l` still empty.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
