# `procoder doctor` under `method = "bmad"` names BMad's installed version, and says plainly that it is absent when it is.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

A repository whose planning depends on a tool procoder does not ship
needs to know that tool is there, and which version — because BMad's
artifact layout is what procoder reads, and a layout change is a version
fact.

Done means doctor names the installed version, and says plainly that it
is absent when it is.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder doctor` under `method = "bmad"` names BMad's installed version, and says plainly that it is absent when it is.

## Evidence

- `go test ./internal/planning/ -run TestSprintStateComesFromTheArtifactsOnDisk` asserts `Version` reads `6.11.0` from the installation's manifest rather than guessing. Verified end to end: `procoder doctor` printed `ok bmad [planning] method 6.11.0` with the install present, and a `GAP` line naming the install command when absent. Silent under the default method — a repository planning in procoder's own chain has no external tool to report.
