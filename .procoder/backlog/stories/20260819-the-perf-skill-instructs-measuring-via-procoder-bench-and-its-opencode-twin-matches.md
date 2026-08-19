# The perf skill instructs measuring via `procoder bench` and its OpenCode twin matches.

Status: done 2026-08-19
Created: 2026-08-19
Epic: perf-bench
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The skill now drives the harness.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The perf skill instructs measuring via `procoder bench` and its OpenCode twin matches.

## Evidence

- commands/perf.md rewritten around `procoder bench` (baseline first, measure again, save deliberately); .opencode/command/perf.md regenerated; twin-parity test green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
