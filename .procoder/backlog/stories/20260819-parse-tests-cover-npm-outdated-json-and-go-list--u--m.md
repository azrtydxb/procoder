# Parse tests cover npm outdated JSON and go list -u -m output shapes, including the everything-current case.

Status: done 2026-08-19
Created: 2026-08-19
Epic: deps-freshness
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The parsers are pinned by recorded shapes.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Parse tests cover npm outdated JSON and go list -u -m output shapes, including the everything-current case.

## Evidence

- TestParseGoList (+EverythingCurrent, indirect, replace), TestParseNpmOutdated (+Empty, +Unparseable, +ErrorObject), TestParsePipOutdated (+Empty/Unparseable), TestMajorBehind (9 cases) all green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
