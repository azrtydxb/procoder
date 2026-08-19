# On procoder's own repo, `procoder deps` prints a Go section with real freshness rows (or an explicit up-to-date line) and a licenses line that is either a go-licenses report or an honest NOT-checked with the install hint.

Status: done 2026-08-19
Created: 2026-08-19
Epic: deps-freshness
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Live on this repository.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On procoder's own repo, `procoder deps` prints a Go section with real freshness rows (or an explicit up-to-date line) and a licenses line that is either a go-licenses report or an honest NOT-checked with the install hint.

## Evidence

- `procoder deps` printed: go up to date, js up to date, licenses (go) NOT checked with the go-licenses install hint, licenses (js) NOT checked, summary 0 behind across 0 ecosystems.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
