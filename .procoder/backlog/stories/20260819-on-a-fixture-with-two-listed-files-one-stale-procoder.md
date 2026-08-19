# On a fixture with two listed files, one stale, `procoder release 1.2.3` lists exactly the stale file, the missing changelog heading, and the dirty tree — all in one output — and exits 1.

Status: done 2026-08-19
Created: 2026-08-19
Epic: release-discipline
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Every failure in one output.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture with two listed files, one stale, `procoder release 1.2.3` lists exactly the stale file, the missing changelog heading, and the dirty tree — all in one output — and exits 1.

## Evidence

- TestAllFailuresReportedTogether: stale file + missing changelog heading + dirty tree listed together, fresh file not flagged, exit 1. Live: `procoder release 0.28.0` pre-bump listed every stale manifest.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
