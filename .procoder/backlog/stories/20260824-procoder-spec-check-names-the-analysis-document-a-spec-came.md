# `procoder spec check` names the analysis document a spec came from when one exists, and does not require one when it does not.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The point of the analysis phase is that a spec comes from somewhere. A
reader asking why a spec says what it says should be able to follow the
link back, without the chain becoming a new tollgate for people who
already know what they are building.

Done means `spec check` names the analysis document when one exists and
stays silent when none does — analysis is available, never required.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder spec check` names the analysis document a spec came from when one exists, and does not require one when it does not.

## Evidence

- `go test ./internal/analysis/ -run TestASpecNamesItsAnalysisOnlyWhenThereIsOne` — SpecSource returns the path when an analysis shares the spec's name and "" when none does; `spec check` prints it as a note. Mutation proven: returning the path unconditionally makes every spec claim an analysis, including specs with no such file.
