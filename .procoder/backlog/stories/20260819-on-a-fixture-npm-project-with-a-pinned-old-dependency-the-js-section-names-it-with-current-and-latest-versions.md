# On a fixture npm project with a pinned old dependency, the JS section names it with current and latest versions.

Status: done 2026-08-19
Created: 2026-08-19
Epic: deps-freshness
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

npm freshness rows parse with current and latest.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture npm project with a pinned old dependency, the JS section names it with current and latest versions.

## Evidence

- TestParseNpmOutdated covers pinned-old and missing packages with current/wanted/latest; the exit-1-with-JSON npm behaviour is treated as an answer.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
