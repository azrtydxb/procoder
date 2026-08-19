# On a fixture with one benchmark, `bench --save` writes the baseline with the header; a second run reports ~0% delta and exits 0.

Status: done 2026-08-19
Created: 2026-08-19
Epic: perf-bench
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Save then compare on a live module.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture with one benchmark, `bench --save` writes the baseline with the header; a second run reports ~0% delta and exits 0.

## Evidence

- TestLiveSaveThenCompare: temp module, --save writes the baseline with the date/commit/GOOS header, second run compares ~0% and exits 0.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
